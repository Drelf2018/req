package req

import (
	"io"
	"reflect"
	"strings"
	"sync"
	"unsafe"
)

type Field struct {
	// index in Api struct
	Index []int
	// provided by "req" tag or parsed from the field's name
	Name string
	// default value used when the field's value is zero
	Value string
	// This field is ignored when it is zero, conflict with default value
	Omit bool
}

type Task struct {
	Body   []Field
	Files  []Field
	Query  []Field
	Header []Field
}

var ioReader = reflect.TypeFor[io.Reader]()

func (task *Task) parse(typ reflect.Type, index []int, parentTag string) {
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)

		if !field.IsExported() {
			continue
		}

		if field.Anonymous && field.Type.Kind() == reflect.Struct {
			task.parse(field.Type, append(index, field.Index[0]), field.Tag.Get("api"))
			continue
		}

		api, ok := field.Tag.Lookup("api")
		if !ok {
			if parentTag == "" {
				continue
			}
			api = parentTag
		}

		var v Field
		api, v.Value, _ = strings.Cut(api, ";")
		api, v.Omit = strings.CutSuffix(api, ",omitempty")

		if api == "files" && !field.Type.Implements(ioReader) {
			continue
		}

		v.Index = append(index, field.Index[0])
		v.Name, ok = field.Tag.Lookup("req")
		if !ok {
			if api == "header" {
				v.Name = HeaderReplace(field.Name)
			} else if field.Name == strings.ToUpper(field.Name) {
				v.Name = strings.ToLower(field.Name)
			} else {
				v.Name = NameReplace(field.Name)
			}
		}

		switch api {
		case "body":
			task.Body = append(task.Body, v)
		case "files":
			task.Files = append(task.Files, v)
		case "query":
			task.Query = append(task.Query, v)
		case "header":
			task.Header = append(task.Header, v)
		}
	}
}

func NewTask(api Api) *Task {
	typ := reflect.TypeOf(api)
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	var task Task
	if typ.Kind() == reflect.Struct {
		if v, ok := api.(ApiTag); ok {
			task.parse(typ, nil, v.ApiTag())
		} else {
			task.parse(typ, nil, "")
		}
	}
	if v, ok := api.(AfterNewTask); ok {
		v.AfterNewTask(&task)
	}
	return &task
}

var taskCache sync.Map

func LoadTask(api Api) *Task {
	ptr := TypePtr(api)
	if v, ok := taskCache.Load(ptr); ok {
		return v.(*Task)
	}
	task := NewTask(api)
	taskCache.Store(ptr, task)
	return task
}

func PreloadTask(api ...Api) {
	for _, i := range api {
		taskCache.Store(TypePtr(i), NewTask(i))
	}
}

type Any struct {
	Type  unsafe.Pointer
	Value unsafe.Pointer
}

func TypePtr(in any) uintptr {
	return uintptr((*Any)(unsafe.Pointer(&in)).Type)
}

// notice this (user is an arbitrary struct)
//
//	TypePtr(user{}) === ValuePtr(reflect.TypeFor[user]())
func ValuePtr(in any) uintptr {
	return uintptr((*Any)(unsafe.Pointer(&in)).Value)
}
