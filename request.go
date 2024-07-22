package req

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"sync"
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

func (f *Field) GetValue(v reflect.Value, whySafe string) reflect.Value {
	for _, i := range f.Index {
		v = v.Field(i)
	}
	return v
}

type Task struct {
	Body   []Field
	Files  []Field
	Query  []Field
	Header []Field
}

var ioReader = reflect.TypeFor[io.Reader]()

func (task *Task) newField(field reflect.StructField, index []int) (api string, v Field) {
	if !field.IsExported() {
		return
	}

	if field.Anonymous && field.Type.Kind() == reflect.Struct {
		task.newTask(field.Type, append(index, field.Index[0]))
		return
	}

	api, ok := field.Tag.Lookup("api")
	if !ok {
		return
	}
	v.Key, ok = field.Tag.Lookup("json")
	if !ok {
		v.Key = Replace(field.Name)
	}
	api, v.Value, _ = strings.Cut(api, ";")
	api, v.Omit = strings.CutSuffix(api, ",omitempty")

	switch api {
	case "files":
		if !field.Type.Implements(ioReader) {
			api = ""
			return
		}
		fallthrough
	case "body", "query", "header":
		v.Index = append(index, field.Index[0])
	}
	return
}

func (task *Task) newTask(typ reflect.Type, index []int) {
	for i := 0; i < typ.NumField(); i++ {
		api, field := task.newField(typ.Field(i), index)
		switch api {
		case "body":
			task.Body = append(task.Body, field)
		case "files":
			task.Files = append(task.Files, field)
		case "query":
			task.Query = append(task.Query, field)
		case "header":
			task.Header = append(task.Header, field)
		}
	}
}

var taskCache sync.Map

func NewTask(typ reflect.Type) *Task {
	var task Task
	task.newTask(typ, nil)
	return &task
}

func LoadTask(in any) *Task {
	ptr := TypePtr(in)
	if v, ok := taskCache.Load(ptr); ok {
		return v.(*Task)
	}
	task := NewTask(reflect.TypeOf(in))
	taskCache.Store(ptr, task)
	return task
}

func LoadTaskByType(typ reflect.Type) *Task {
	ptr := ValuePtr(typ)
	if v, ok := taskCache.Load(ptr); ok {
		return v.(*Task)
	}
	task := NewTask(typ)
	taskCache.Store(ptr, task)
	return task
}

func LoadTaskByPtr(ptr uintptr) (task *Task, ok bool) {
	v, ok := taskCache.Load(ptr)
	if ok {
		task = v.(*Task)
	}
	return
}

func AddData(v Adder, fields []Field, api reflect.Value) error {
	for _, data := range fields {
		field := data.GetValue(api, "have the same reflect.Type")
		if field.IsZero() {
			if !data.Omit {
				v.Add(data.Key, data.Value)
			}
			continue
		}

		switch field.Kind() {
		case reflect.Array, reflect.Slice:
			for i := 0; i < field.Len(); i++ {
				s, err := Marshal(field.Index(i).Interface())
				if err != nil {
					return err
				}
				v.Add(data.Key, s)
			}
		default:
			s, err := Marshal(field.Interface())
			if err != nil {
				return err
			}
			v.Add(data.Key, s)
		}
	}
	return nil
}

const UserAgent string = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/116.0.0.0 Safari/537.36 Edg/116.0.1938.54"

func NewRequest(api Api) (req *http.Request, err error) {
	return NewRequestWithContext(context.Background(), api)
}

func NewRequestWithContext(ctx context.Context, api Api) (req *http.Request, err error) {
	v := reflect.ValueOf(api)
	task := LoadTask(api)

	if task.Body == nil && task.Files == nil {
		req, err = http.NewRequestWithContext(ctx, api.Method(), api.URL(), nil)
	} else if task.Files != nil {
		w, r := NewPipe()
		for _, data := range task.Files {
			field := data.GetValue(v, "have the same reflect.Type")
			if field.IsZero() {
				continue
			}
			switch file := field.Interface().(type) {
			case NamedReader:
				err = w.WriteFile(data.Key, file.Name(), file)
			case io.Reader:
				err = w.WriteFile(data.Key, data.Value, file)
			}
			if err != nil {
				return
			}
		}
		err = AddData(w, task.Body, v)
		if err != nil {
			return
		}
		err = w.Close()
		if err != nil {
			return
		}
		req, err = http.NewRequestWithContext(ctx, api.Method(), api.URL(), r)
		req.Header.Set("Content-Type", w.ContentType())
	} else if _, isJson := api.(postJson); isJson {
		body := make(JsonBody)
		err = AddData(body, task.Body, v)
		if err != nil {
			return
		}
		buf := &bytes.Buffer{}
		err = json.NewEncoder(buf).Encode(body)
		if err != nil {
			return
		}
		req, err = http.NewRequestWithContext(ctx, api.Method(), api.URL(), buf)
	} else {
		body := make(url.Values)
		err = AddData(body, task.Body, v)
		if err != nil {
			return
		}
		req, err = http.NewRequestWithContext(ctx, api.Method(), api.URL(), strings.NewReader(body.Encode()))
	}

	if err != nil {
		return
	}

	if task.Query != nil {
		query := make(url.Values)
		err = AddData(query, task.Query, v)
		if err != nil {
			return
		}
		req.URL.RawQuery = query.Encode()
	}

	req.Header.Add("User-Agent", UserAgent)
	err = AddData(req.Header, task.Header, v)

	return
}
