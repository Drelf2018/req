package req

import (
	"reflect"
	"strings"
	"sync"
	"unsafe"
)

type Any struct {
	Type  unsafe.Pointer
	Value unsafe.Pointer
}

func TypePtr(in any) uintptr {
	return uintptr((*Any)(unsafe.Pointer(&in)).Type)
}

// ValuePtr can obtain the uintptr of a type from its reflect.Type
//
//	TypePtr(something{}) is equal to ValuePtr(reflect.TypeOf(something{}))
func ValuePtr(in any) uintptr {
	return uintptr((*Any)(unsafe.Pointer(&in)).Value)
}

// 字段
type Field struct {
	// 字段在结构体中的索引，因为结构体可以嵌套所以有多层
	Index []int

	// 字段要使用的名称，例如 json 中 key 的部分
	Name string

	// 字段的默认值
	Value string

	// 字段为空时是否忽略这项
	Omitempty bool
}

// 任务
//
// 包含了 API 结构体中所有有效字段信息
type Task struct {
	Body   []Field
	Query  []Field
	Header []Field
	Cookie []Field
}

func (task *Task) parse(typ reflect.Type, index []int, parentTag string) {
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if !field.IsExported() {
			continue
		}

		api := field.Tag.Get("api")
		if api == "-" {
			continue
		}

		if field.Anonymous {
			fieldType := field.Type
			if fieldType.Kind() == reflect.Pointer {
				fieldType = fieldType.Elem()
			}
			if fieldType.Kind() == reflect.Struct {
				task.parse(fieldType, append(index, field.Index[0]), api)
			}
			continue
		}

		if api == "" {
			if parentTag == "" {
				continue
			}
			api = parentTag
		}

		var v Field // tag:"api:value,omitempty"
		api, _, v.Omitempty = strings.Cut(api, ",omitempty")
		api, v.Value, _ = strings.Cut(api, ":")

		v.Index = append(index, field.Index[0])

		if req, ok := field.Tag.Lookup("req"); ok {
			v.Name = req
		} else if api == "header" {
			v.Name = HeaderReplace(field.Name)
		} else if field.Name == strings.ToUpper(field.Name) {
			// because NameReplace(OS/URL/MIME/HTML/CSS) => o_s/u_r_l/m_i_m_e/h_t_m_l/c_s_s
			// but we just want os/url/mime/html/css
			v.Name = strings.ToLower(field.Name)
		} else {
			v.Name = NameReplace(field.Name)
		}

		switch api {
		case "body":
			task.Body = append(task.Body, v)
		case "query":
			task.Query = append(task.Query, v)
		case "header":
			task.Header = append(task.Header, v)
		case "cookie":
			task.Cookie = append(task.Cookie, v)
		}
	}
}

// 新建任务
func NewTask(api API) *Task {
	var task Task
	apiType := reflect.TypeOf(api)
	if apiType.Kind() == reflect.Pointer {
		apiType = apiType.Elem()
	}
	if apiType.Kind() == reflect.Struct {
		task.parse(apiType, nil, "")
	}
	return &task
}

var taskCache sync.Map // map[uintptr]*Task

// 根据 API 加载任务
//
// 不存在则新建
func LoadTask(api API) *Task {
	ptr := TypePtr(api)
	if v, ok := taskCache.Load(ptr); ok {
		return v.(*Task)
	}
	task := NewTask(api)
	taskCache.Store(ptr, task)
	return task
}

// 预加载任务
func PreloadTask(api ...API) {
	for _, i := range api {
		taskCache.Store(TypePtr(i), NewTask(i))
	}
}
