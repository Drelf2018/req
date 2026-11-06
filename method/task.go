package method

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
)

var (
	ErrMissing     = errors.New("req/method: required but no value resolved")
	ErrNilArgument = errors.New("req/method: nil argument")
)

// Field 描述一个字段的取值链、去向和编码方式
type Field struct {
	Index  []int  // 定位字段
	Name   string // 从字段名转写的参数名称
	Target string // body / query / header / cookie / 自定义等去向

	CtxKey    string  // 从上下文取值的键
	CookieKey string  // 从 Cookie 取值的键
	EnvKey    string  // 从环境变量取值的键
	Default   *string // 默认值，为空时设为字段类型零值

	Omit     bool // 是否包含 ,omitempty 修饰符
	Required bool // 是否包含 ,required 修饰符

	encode encodeFunc // 编码函数，用来直接将 reflect.Value 转成该字段正确的字符串形式
}

// ToString 按“字段 → context → cookie → env → default”的顺序为字段取值，命中即返回。
//
// 字符串目标的形态：字段自身与链尾零值走编译期编码器，context 按动态类型归一化，
// cookie / env / default 原样作为一条字符串。
//
// 取值链与 ToValue 必须保持一致，改动一边就要照看另一边。
func (f *Field) ToString(ctx context.Context, root reflect.Value, reader CookieReader) ([]string, error) {
	// 一般不会有人传入与字段索引不对应的值，这里做个保险
	value, err := root.FieldByIndexErr(f.Index)
	if err != nil {
		return nil, fmt.Errorf("req/method: cannot reach field %q: %w", f.Name, err)
	}

	// 字段自身
	if value.IsValid() && !value.IsZero() {
		return f.encode(value)
	}

	// 上下文
	if f.CtxKey != "" && ctx != nil {
		if v := ctx.Value(f.CtxKey); v != nil {
			return Encode(v)
		}
	}

	// Cookie
	if f.CookieKey != "" && reader != nil {
		c, err := reader.Cookie(f.CookieKey)
		if err == nil {
			return []string{c.Value}, nil
		}
	}

	// 环境变量
	if f.EnvKey != "" {
		if s, ok := os.LookupEnv(f.EnvKey); ok {
			return []string{s}, nil
		}
	}

	// 默认值
	if f.Default != nil {
		return []string{*f.Default}, nil
	}

	// 链尾
	switch {
	case f.Omit:
		return nil, nil
	case f.Required:
		return nil, ErrMissing
	default:
		return f.encode(value)
	}
}

// ToValue 与 ToString 跑同一条取值链，但保留 json 目标需要的类型：
// 字段自身与 context 保留原类型，cookie / env / default 必须转成字段类型。
// 返回 nil 表示“没值”，调用方应当省略这个键，而不是写成 null。
func (f *Field) ToValue(ctx context.Context, root reflect.Value, reader CookieReader) (any, error) {
	// 一般不会有人传入与字段索引不对应的值，这里做个保险
	value, err := root.FieldByIndexErr(f.Index)
	if err != nil {
		return nil, fmt.Errorf("req/method: cannot reach field %q: %w", f.Name, err)
	}

	// 字段自身
	if value.IsValid() && !value.IsZero() {
		return value.Interface(), nil
	}

	// 上下文
	if f.CtxKey != "" && ctx != nil {
		if v := ctx.Value(f.CtxKey); v != nil {
			return v, nil
		}
	}

	// Cookie
	if f.CookieKey != "" && reader != nil {
		c, err := reader.Cookie(f.CookieKey)
		if err == nil {
			return DecodeType(value.Type(), c.Value)
		}
	}

	// 环境变量
	if f.EnvKey != "" {
		if s, ok := os.LookupEnv(f.EnvKey); ok {
			return DecodeType(value.Type(), s)
		}
	}

	// 默认值
	if f.Default != nil {
		return DecodeType(value.Type(), *f.Default)
	}

	// 链尾
	switch {
	case f.Omit:
		return nil, nil
	case f.Required:
		return nil, ErrMissing
	default:
		return value.Interface(), nil
	}
}

// Task 是请求 API 的任务形式，包含了其结构体中的有效字段
type Task struct {
	Body   []Field
	Query  []Field
	Header []Field
	Cookie []Field
	Custom map[string][]Field // 自定义标签
}

// CompileTask 编译 v 的类型对应的请求任务，v 需为非 nil 的结构体或其指针
func CompileTask(v any) (*Task, error) {
	if v == nil {
		return nil, ErrNilArgument
	}
	return compileTask(reflect.TypeOf(v))
}

// Renamer 参数名转换器
type Renamer struct{}

// Rename 将请求头字段名转为连字符式名称，将其他字段名转为下划线式名称
func (Renamer) Rename(target string, name string) string {
	if target == "header" {
		return PascalToHyphenated(name)
	}
	return PascalToSnake(name)
}

// DefaultRenamer 全局参数名转换器
var DefaultRenamer APIRenamer = Renamer{}

// compileTask 解析 API 结构体内有效字段的标签，生成其任务形式
func compileTask(t reflect.Type) (*Task, error) {
	// 解引用得到底层结构体
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("req/method: %v is not a struct", t)
	}
	// 获取其可能注册在指针接收器上的重命名接口
	rename, ok := reflect.New(t).Interface().(APIRenamer)
	if !ok {
		rename = DefaultRenamer
	}
	// 遍历导出字段
	task := &Task{Custom: make(map[string][]Field)}
	for _, vf := range reflect.VisibleFields(t) {
		if !vf.IsExported() {
			continue
		}
		tag := vf.Tag.Get("req")
		if tag == "" {
			continue
		}
		if vf.Anonymous {
			return nil, fmt.Errorf("req/method: anonymous field %q cannot carry a req tag", vf.Name)
		}
		// 标签规则 <target>[:<name>][,<modifier>]...
		// 第一个参数固定为目标和可选的名字，后续用逗号追加零或多个修饰符
		f := Field{}
		args := strings.Split(tag, ",")
		f.Target, f.Name, _ = strings.Cut(args[0], ":")
		if f.Target == "" {
			return nil, fmt.Errorf("req/method: field %q req tag has no target", vf.Name)
		}
		// 重命名字段名
		if f.Name == "" {
			f.Name = rename.Rename(f.Target, vf.Name)
		}
		// 解析修饰符
		for _, mod := range args[1:] {
			switch mod {
			case "omitempty":
				f.Omit = true
			case "required":
				f.Required = true
			default:
				return nil, fmt.Errorf("req/method: field %q has unknown req modifier %q", vf.Name, mod)
			}
		}
		// 省略空值与必须冲突
		if f.Omit && f.Required {
			return nil, fmt.Errorf("req/method: field %q cannot use ,omitempty and ,required together", vf.Name)
		}
		if d, ok := vf.Tag.Lookup("default"); ok {
			f.Default = &d
			// 默认参数与修饰符冲突
			if f.Omit {
				return nil, fmt.Errorf("req/method: default on field %q conflicts with ,omitempty", vf.Name)
			}
			if f.Required {
				return nil, fmt.Errorf("req/method: default on field %q conflicts with ,required", vf.Name)
			}
		}
		// 加载该字段编码器
		encode, err := loadEncode(vf.Type)
		if err != nil {
			return nil, err
		}
		// 设置其他字段
		f.Index = append([]int(nil), vf.Index...)
		f.CtxKey = vf.Tag.Get("ctx")
		f.CookieKey = vf.Tag.Get("cookie")
		f.EnvKey = vf.Tag.Get("env")
		f.encode = encode
		// body 目标且声明了字符串来源时，构建 body 会用到解码器；这里预校验，
		// 把接口这类无法解码的类型变成构建期错误，而不是等某次请求才炸。
		if f.Target == "body" && (f.CookieKey != "" || f.EnvKey != "" || f.Default != nil) {
			if _, err := loadDecode(vf.Type); err != nil {
				return nil, err
			}
		}
		// 分类存储
		switch f.Target {
		case "body":
			task.Body = append(task.Body, f)
		case "query":
			task.Query = append(task.Query, f)
		case "header":
			task.Header = append(task.Header, f)
		case "cookie":
			task.Cookie = append(task.Cookie, f)
		default:
			task.Custom[f.Target] = append(task.Custom[f.Target], f)
		}
	}
	return task, nil
}

// taskCache 请求任务缓存 map[reflect.Type]*Task
var taskCache sync.Map

// LoadTask 根据 API 加载请求任务，如果缓存中不存在则创建新任务并存入缓存
func LoadTask(api API) (*Task, error) {
	t := reflect.TypeOf(api)
	if t == nil {
		return nil, ErrNilArgument
	}
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if v, ok := taskCache.Load(t); ok {
		return v.(*Task), nil
	}
	task, err := compileTask(t)
	if err != nil {
		return nil, err
	}
	taskCache.Store(t, task)
	return task, nil
}

// PreloadTask 预加载多个 API 的请求任务到缓存中
func PreloadTask(api ...API) error {
	for _, i := range api {
		_, err := LoadTask(i)
		if err != nil {
			return err
		}
	}
	return nil
}
