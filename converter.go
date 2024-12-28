package req

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/format"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var ErrInvalidDelim = errors.New("req: invalid delim")
var ErrInvalidToken = errors.New("req: invalid token")

var patternAllNumber = regexp.MustCompile(`^\d+$`)
var pattern2 = regexp.MustCompile(`(^|[^a-zA-Z])([a-z]+)`)
var pattern3 = regexp.MustCompile(`([A-Z])([a-z]+)`)
var pattern4 = regexp.MustCompile(`[^a-zA-Z0-9]`)

var numbers = map[byte]string{
	'0': "Zero_",
	'1': "One_",
	'2': "Two_",
	'3': "Three_",
	'4': "Four_",
	'5': "Five_",
	'6': "Six_",
	'7': "Seven_",
	'8': "Eight_",
	'9': "Nine_",
}

var Void = struct{}{}

// 需要保持全大写的字典 可自行添加
var Abbreviation = map[string]struct{}{
	"API":   Void,
	"ASCII": Void,
	"CPU":   Void,
	"CSS":   Void,
	"DNS":   Void,
	"EOF":   Void,
	"GUID":  Void,
	"HTML":  Void,
	"HTTP":  Void,
	"HTTPS": Void,
	"ID":    Void,
	"IP":    Void,
	"JSON":  Void,
	"LHS":   Void,
	"QPS":   Void,
	"RAM":   Void,
	"RHS":   Void,
	"RPC":   Void,
	"SLA":   Void,
	"SMTP":  Void,
	"SSH":   Void,
	"TCP":   Void,
	"TLS":   Void,
	"TTL":   Void,
	"UDP":   Void,
	"UI":    Void,
	"UID":   Void,
	"UUID":  Void,
	"URI":   Void,
	"URL":   Void,
	"UTF8":  Void,
	"VM":    Void,
	"XML":   Void,
	"XSRF":  Void,
	"XSS":   Void,
}

// 仿 JavaScript 的字符串替换函数
func JavaScriptReplace(s string, pattern *regexp.Regexp, fn func(match string, submatch ...string) string) string {
	idx := pattern.FindAllStringSubmatchIndex(s, -1)
	if len(idx) == 0 {
		return s
	}
	buf := &bytes.Buffer{}
	for i, v := range pattern.FindAllStringSubmatch(s, -1) {
		if i == 0 {
			buf.WriteString(s[:idx[i][0]])
		} else {
			buf.WriteString(s[idx[i-1][1]:idx[i][0]])
		}
		buf.WriteString(fn(v[0], v[1:]...))
	}
	buf.WriteString(s[idx[len(idx)-1][1]:])
	return buf.String()
}

// 规范化名称
func RegularizeName(name string) string {
	name = JavaScriptReplace(name, pattern2, func(match string, submatch ...string) string {
		n := submatch[0]
		r := submatch[1]
		if _, ok := Abbreviation[strings.ToUpper(r)]; ok {
			return n + strings.ToUpper(r)
		} else {
			return n + strings.ToUpper(r[:1]) + strings.ToLower(r[1:])
		}
	})
	name = JavaScriptReplace(name, pattern3, func(match string, submatch ...string) string {
		n := submatch[0]
		r := submatch[1]
		if _, ok := Abbreviation[n+strings.ToUpper(r)]; ok {
			return strings.ToUpper(n + r)
		} else {
			return n + r
		}
	})
	return pattern4.ReplaceAllString(name, "")
}

// 正规化名称
func NormalizeName(name string) string {
	if name == "" {
		return ""
	}
	if patternAllNumber.MatchString(name) {
		name = "Num" + name
	} else if name[0] == '-' && patternAllNumber.MatchString(name[1:]) {
		name = "Neg" + name[1:]
	} else if prefix, ok := numbers[name[0]]; ok {
		name = prefix + name[1:]
	}

	name = RegularizeName(name)
	if name != "" {
		return name
	}
	return "NamingFailed"
}

// 解析 array 中类型名
//
// 因为 json 的 array 里面允许不同类型的值，所以需要一个函数来判断到底用哪个
// 如果都是数字就取范围最大的那个，如果有非数字类型则用 any
func ParseNumberType(type1, type2 string) string {
	switch {
	case strings.HasPrefix(type1, "float"):
		if strings.HasPrefix(type2, "int") {
			return type1
		} else if strings.HasPrefix(type2, "float") {
			return "float64"
		}
	case strings.HasPrefix(type1, "int"):
		if strings.HasPrefix(type2, "float") {
			return type2
		} else if strings.HasPrefix(type2, "int") {
			return "int64"
		}
	}
	return "any"
}

// 解析类型名
func ParseType(t any) (rtype string, comment string) {
	switch t := t.(type) {
	case json.Delim:
		switch t {
		case '{':
			return "struct", fmt.Sprint(t)
		case '[':
			return "slice", fmt.Sprint(t)
		}
	case bool:
		if t {
			return "bool", "true"
		} else {
			return "bool", "false"
		}
	case float64:
		return "float64", strconv.FormatFloat(t, 'f', -1, 32)
	case json.Number:
		comment = t.String()
		if !strings.Contains(comment, ".") {
			i, errAtoi := strconv.Atoi(comment)
			if errAtoi == nil {
				if i >= -2147483648 && i <= 2147483647 {
					rtype = "int"
				} else {
					rtype = "int64"
				}
				return
			}
		}
		_, errParse := strconv.ParseFloat(comment, 64)
		if errParse == nil {
			rtype = "float64"
		} else {
			rtype = "json.Number"
		}
		return
	case string:
		comment = "\"" + t + "\""
		if new(time.Time).UnmarshalText([]byte(t)) == nil {
			rtype = "time.Time"
		} else {
			rtype = "string"
		}
		return
	case []any:
		return "slice", ""
	case []*object:
		return "struct", ""
	}
	return "any", ""
}

// 单个字段
//
// 这个结构体是为了实现有序的字典(map[string]any)创建的
//
// 众所周知 golang 的字典是无序的，因此为了保证导出的字段顺序与输入一致
// 用了这个结构体的列表([]*object)来描述字典
type object struct {
	// 键名
	key string

	// 类型名
	rtype string

	// 是否在 json 标签中添加 ",omitempty"
	omit bool

	// 用来统计 array 中的 object 的这个键出现了几次
	// 如果出现次数等于 array 长度则代表不需要 omit
	count int

	// 值 可能的类型有 json.Token / []*object / []any
	value any

	// 注释 就是原本的值字符串
	comment string
}

func (o object) String() string {
	return fmt.Sprintf("%s:%s", o.key, o.rtype)
}

func (o *object) Objects() []*object {
	return o.value.([]*object)
}

func (o *object) Array() []any {
	return o.value.([]any)
}

// 转换器
type Converter struct {
	// 是否使用 any 代替 interface{}
	Any bool

	// 是否在字段后加注释
	Comment bool

	buf *bytes.Buffer
	dec *json.Decoder
	tab int
}

func (c *Converter) Add(s string) {
	c.buf.WriteString(s)
}

func (c *Converter) AddTab() {
	for i := 0; i < c.tab; i++ {
		c.buf.WriteByte('\t')
	}
}

// 将 JSON 转换成结构体字符串
//
// 参数 name 为最外层结构体名字（会做修改）
func (c *Converter) JSONToStruct(b []byte, name string) ([]byte, error) {
	c.tab = 0
	c.buf.Reset()
	name = NormalizeName(name)
	if name == "" {
		name = "AutoGenerated"
	}
	c.Add("type ")
	c.Add(name)
	c.Add(" ")
	err := c.UnmarshalJSON(bytes.ReplaceAll(b, []byte(".0"), []byte(".1")))
	if err != nil {
		return nil, err
	}
	return format.Source(c.buf.Bytes())
}

func assert(dec *json.Decoder, token json.Delim) error {
	t, err := dec.Token()
	if err != nil {
		return err
	}
	if t, ok := t.(json.Delim); !ok || t != token {
		return ErrInvalidDelim
	}
	return nil
}

func (c *Converter) UnmarshalJSON(data []byte) error {
	c.dec = json.NewDecoder(bytes.NewReader(data))
	c.dec.UseNumber()

	err := assert(c.dec, '{')
	if err != nil {
		return err
	}

	objects, err := c.ParseObject([]*object{})
	if err != nil {
		return err
	}

	_, err = c.dec.Token()
	if err != io.EOF {
		return err
	}

	return c.WriteObjects(objects)
}

// 解析对象
func (c *Converter) ParseObject(objects []*object) ([]*object, error) {
outer:
	for c.dec.More() {
		t, err := c.dec.Token()
		if err != nil {
			return nil, err
		}

		key, ok := t.(string)
		if !ok {
			return nil, ErrInvalidToken
		}

		// 遍历这个列表
		// 如果已经保存过该字段 计数加 1
		// 直接读取并舍弃它的 Value
		for _, obj := range objects {
			if obj.key == key {
				obj.count++
				_, err := c.ParseValue(nil)
				if err != nil {
					return nil, err
				}
				continue outer
			}
		}

		// 否则获取值并解析类型
		// 再添加进 objects 列表
		value, err := c.ParseValue([]*object{})
		if err != nil {
			return nil, err
		}

		rtype, comment := ParseType(value)
		if rtype == "any" && !c.Any {
			rtype = "interface{}"
		}

		if objects != nil {
			objects = append(objects, &object{
				key:     key,
				rtype:   rtype,
				count:   1,
				value:   value,
				comment: comment,
			})
		}
	}

	// 劲爆尾杀
	return objects, assert(c.dec, '}')
}

// 解析值
func (c *Converter) ParseValue(objects []*object) (any, error) {
	t, err := c.dec.Token()
	if err != nil {
		return nil, err
	}
	if t, ok := t.(json.Delim); ok {
		switch t {
		case '{':
			// 以大括号开启代表接下来的值是一个对象
			// 调用对应方法
			return c.ParseObject(objects)
		case '[':
			// 同理调用解析数组的方法
			return c.ParseArray()
		}
	}
	return t, nil
}

// 解析数组
func (c *Converter) ParseArray() (val []any, err error) {
	// 计数组长度
	length := 0
	// 数组中可能存在的对象
	objects := make([]*object, 0)
	for c.dec.More() {
		length++
		v, err := c.ParseValue(objects)
		if err != nil {
			return nil, err
		}
		if objs, ok := v.([]*object); ok {
			// 如果包含对象则赋值
			// 类似 append 方法
			objects = objs
		} else {
			// 否则在返回值中添加解析出来的值
			val = append(val, v)
		}
	}
	// 劲爆尾杀
	err = assert(c.dec, ']')
	if err != nil {
		return
	}
	// 列表中没东西有两种情况
	// 一种是真没内容 => []any
	// 还有一种是里面有对象保存在 objects 中而非 val 中
	if len(val) == 0 {
		for _, obj := range objects {
			// 处理每一个字段的 omit
			// 原理参考 count 字段上的注释
			obj.omit = obj.count != length
		}
	}
	// 对象非空 添加进列表的值中
	if len(objects) != 0 {
		val = append(val, objects)
	}
	return
}

// 向 Buffer 写入结构体字符串
func (c *Converter) WriteObjects(objects []*object) (err error) {
	c.Add("struct {\n")
	c.tab++
	for _, obj := range objects {
		// 如果注释中有换行符 那么就写在字段的上方 并且每个换行都是注释
		hasNewLine := strings.Contains(obj.comment, "\n")
		if c.Comment && hasNewLine {
			for _, comment := range strings.Split(obj.comment, "\n") {
				c.AddTab()
				c.Add("// ")
				c.Add(comment)
				c.Add("\n")
			}
		}
		// 写入正规化的字段名
		c.AddTab()
		c.Add(NormalizeName(obj.key))
		c.Add(" ")
		// 根据该字段类型决定接下来如何写入
		switch obj.rtype {
		case "struct":
			err = c.WriteObjects(obj.Objects())
		case "slice":
			err = c.WriteArray(obj.Array())
		default:
			c.Add(obj.rtype)
		}
		if err != nil {
			return nil
		}
		// 写入 json 标签
		c.Add(" `json:\"")
		c.Add(obj.key)
		if obj.omit {
			c.Add(",omitempty")
		}
		c.Add("\"`")
		// 写入注释
		if c.Comment && obj.comment != "" && !hasNewLine {
			c.Add(" // ")
			c.Add(obj.comment)
		}
		c.Add("\n")
	}
	// 细节闭合括号
	c.tab--
	c.AddTab()
	c.Add("}")
	return nil
}

// 向 Buffer 写入数组字符串
func (c *Converter) WriteArray(val []any) (err error) {
	c.Add("[]")

	// 对象数组 []struct
	if len(val) == 1 {
		if o, ok := val[0].([]*object); ok {
			err = c.WriteObjects(o)
			return
		}
	}

	// 非对象数组 []int / []string / [][]int / ...
	var rtype string
	for idx := 0; idx < len(val); idx++ {
		ntype, _ := ParseType(val[idx])
		if rtype == "" {
			rtype = ntype
		} else if rtype != ntype {
			rtype = ParseNumberType(ntype, rtype)
			if rtype == "any" {
				break
			}
		}
	}

	if rtype == "slice" {
		c.WriteArray(val[0].([]any))
		return
	}

	if rtype == "" {
		rtype = "any"
	}
	if rtype == "any" && !c.Any {
		rtype = "interface{}"
	}
	c.Add(rtype)
	return nil
}

func (c *Converter) String() string {
	return c.buf.String()
}

// 冷知识 any 其实不是关键字 是可以重写的
func NewConverter(any bool, comment bool) *Converter {
	return &Converter{
		Any:     any,
		Comment: comment,
		buf:     &bytes.Buffer{},
	}
}
