package method

import (
	"reflect"
	"strings"
	"sync"
	"unsafe"
)

// 请求任务
//
// 包含了 API 结构体中所有有效字段信息
type Task struct {
	Body   []reflect.StructField
	Query  []reflect.StructField
	Header []reflect.StructField
	Cookie []reflect.StructField
	Custom map[string][]reflect.StructField // 自定义标签
}

// NewTask 根据给定的结构体类型生成请求任务
func NewTask(v any) (task Task) {
	for _, f := range reflect.VisibleFields(reflect.TypeOf(v)) {
		tag := f.Tag.Get("req")
		if tag == "" {
			continue
		}
		// 分解标签 `req:"param[:name][,omitempty]" default:"value"`
		tag, _, omit := strings.Cut(tag, ",omitempty")
		tag, name, _ := strings.Cut(tag, ":")
		// 忽略零值和默认值
		if omit {
			f.Tag = reflect.StructTag("")
		}
		// 生成字段名
		if name != "" {
			f.Name = name
		} else if tag == "header" {
			f.Name = HeaderReplacer(f.Name)
		} else {
			f.Name = NameReplacer(f.Name)
		}
		// 分类存储字段
		switch tag {
		case "body":
			task.Body = append(task.Body, f)
		case "query":
			task.Query = append(task.Query, f)
		case "header":
			task.Header = append(task.Header, f)
		case "cookie":
			task.Cookie = append(task.Cookie, f)
		default:
			if task.Custom == nil {
				task.Custom = make(map[string][]reflect.StructField)
			}
			task.Custom[tag] = append(task.Custom[tag], f)
		}
	}
	return
}

// Emptyface 是空接口的内部表示，用于获取类型和值指针
type Emptyface struct {
	Type  unsafe.Pointer
	Value unsafe.Pointer
}

// TypePtr 获取任意类型变量的类型指针，用于唯一标识该类型
func TypePtr(in any) uintptr {
	return uintptr((*Emptyface)(unsafe.Pointer(&in)).Type)
}

// ValuePtr 获取任意类型变量的值指针，用于唯一标识该变量
func ValuePtr(in any) uintptr {
	return uintptr((*Emptyface)(unsafe.Pointer(&in)).Value)
}

// taskCache 请求任务缓存 map[uintptr]Task
var taskCache sync.Map

// LoadTask 根据 API 加载请求任务，如果缓存中不存在则创建新任务并存入缓存
func LoadTask(api any) Task {
	ptr := TypePtr(api)
	if v, ok := taskCache.Load(ptr); ok {
		return v.(Task)
	}
	task := NewTask(api)
	taskCache.Store(ptr, task)
	return task
}

// PreloadTask 预加载多个 API 的请求任务到缓存中
func PreloadTask(api ...any) {
	for _, i := range api {
		taskCache.Store(TypePtr(i), NewTask(i))
	}
}
