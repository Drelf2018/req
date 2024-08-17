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
	// provided by json tag or parsed from the field name
	Key string
	// default value used when the field value is zero
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

func (task *Task) Parse(typ reflect.Type, index []int) {
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)

		if !field.IsExported() {
			continue
		}

		if field.Anonymous && field.Type.Kind() == reflect.Struct {
			task.Parse(field.Type, append(index, field.Index[0]))
			continue
		}

		var v Field
		api, ok := field.Tag.Lookup("api")
		if !ok {
			continue
		}
		api, v.Value, _ = strings.Cut(api, ";")
		api, v.Omit = strings.CutSuffix(api, ",omitempty")

		if api == "files" && !field.Type.Implements(ioReader) {
			continue
		}

		v.Key, ok = field.Tag.Lookup("json")
		if !ok {
			if api == "header" {
				v.Key = HeaderReplace(field.Name)
			} else {
				v.Key = KeyReplace(field.Name)
			}
		}

		v.Index = append(index, field.Index[0])

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

func NewTask(typ reflect.Type) *Task {
	var task Task
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ.Kind() == reflect.Struct {
		task.Parse(typ, nil)
	}
	return &task
}

var taskCache sync.Map

func LoadTask(in any) *Task {
	ptr := TypePtr(in)
	if v, ok := taskCache.Load(ptr); ok {
		return v.(*Task)
	}
	task := NewTask(reflect.TypeOf(in))
	taskCache.Store(ptr, task)
	return task
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
//	TypePtr(user{}) == ValuePtr(reflect.TypeFor[user]())
func ValuePtr(in any) uintptr {
	return uintptr((*Any)(unsafe.Pointer(&in)).Value)
}
