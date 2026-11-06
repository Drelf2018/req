package cookie

import (
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"sync"
	"unsafe"
)

var (
	ErrNilValue = errors.New("req/cookie: nil value")
	ErrNilType  = errors.New("req/cookie: nil type")
)

var cookieCache sync.Map // map[reflect.Type][]cookieField

// cookieField 保存有 cookie 标签的字段信息
type cookieField struct {
	name   string
	offset uintptr
	embed  bool
}

// base 取 rv 指向对象的基址，其值不是非空指针时返回错误
func base(rv reflect.Value) (unsafe.Pointer, error) {
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return nil, fmt.Errorf("req/cookie: want non-nil pointer, got %s", rv.Type())
	}
	return rv.UnsafePointer(), nil
}

// loadCookie 获取结构体类型对应的 Cookie 偏移表，类型为空或解引用后不是结构体时返回错误
func loadCookie(t reflect.Type) ([]cookieField, error) {
	if t == nil {
		return nil, ErrNilType
	}
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("req/cookie: non-struct type: %s", t)
	}
	// 如果已经生成过偏移表，直接返回
	if value, ok := cookieCache.Load(t); ok {
		return value.([]cookieField), nil
	}
	// 广度优先遍历对象的字段
	fields := make([]cookieField, 0)
	structs := []reflect.StructField{{Type: t, Offset: 0}}
	for i := 0; i < len(structs); i++ {
		elem := structs[i].Type
		offset := structs[i].Offset
		for j := 0; j < elem.NumField(); j++ {
			field := elem.Field(j)
			if !field.IsExported() {
				continue
			}
			// 字符串字段设置了标签则创建映射
			// 结构体字段添加进遍历切片
			// 映射字段设置了嵌入标签则用于存取未显示定义的键值
			switch field.Type.Kind() {
			case reflect.String:
				if tag, ok := field.Tag.Lookup("cookie"); ok {
					fields = append(fields, cookieField{
						name:   tag,
						offset: offset + field.Offset,
					})
				}
			case reflect.Struct:
				structs = append(structs, reflect.StructField{
					Type:   field.Type,
					Offset: offset + field.Offset,
				})
			case reflect.Map:
				if field.Tag.Get("cookie") == ",embed" {
					if field.Type.Key().Kind() != reflect.String || field.Type.Elem().Kind() != reflect.String {
						return nil, fmt.Errorf("req/cookie: %s.%s: embed option requires map[string]string, got %v", t, field.Name, field.Type)
					}
					fields = append(fields, cookieField{
						offset: offset + field.Offset,
						embed:  true,
					})
				}
			}
		}
	}
	cookieCache.Store(t, fields)
	return fields, nil
}

// Get 从结构体中获取 Cookie 切片，当传入参数为空、非结构体或其指针时返回错误
func Get(v any) ([]*http.Cookie, error) {
	if v == nil {
		return nil, ErrNilValue
	}
	rv := reflect.ValueOf(v)
	ptr, err := base(rv)
	if err != nil {
		return nil, err
	}
	fields, err := loadCookie(rv.Type())
	if err != nil {
		return nil, err
	}
	cookies := make([]*http.Cookie, 0, len(fields))
	// 第一趟只输出声明字段，并记下名字
	seen := make(map[string]bool, len(fields))
	for _, field := range fields {
		if field.embed {
			continue
		}
		seen[field.name] = true
		cookies = append(cookies, &http.Cookie{Name: field.name, Value: *(*string)(unsafe.Add(ptr, field.offset))})
	}
	// 第二趟补兜底槽里没被声明过的键值
	for _, field := range fields {
		if !field.embed {
			continue
		}
		for name, value := range *(*map[string]string)(unsafe.Add(ptr, field.offset)) {
			if !seen[name] {
				seen[name] = true
				cookies = append(cookies, &http.Cookie{Name: name, Value: value})
			}
		}
	}
	return cookies, nil
}

// Set 将 Cookie 切片设置到结构体中，当传入参数为空、非结构体或其指针时返回错误
func Set(v any, cookies []*http.Cookie) error {
	if v == nil {
		return ErrNilValue
	}
	rv := reflect.ValueOf(v)
	ptr, err := base(rv)
	if err != nil {
		return err
	}
	fields, err := loadCookie(rv.Type())
	if err != nil {
		return err
	}
	var rest *map[string]string
	for _, field := range fields {
		if field.embed {
			rest = (*map[string]string)(unsafe.Add(ptr, field.offset))
			break
		}
	}
	for _, cookie := range cookies {
		matched := false
		for _, field := range fields {
			if !field.embed && field.name == cookie.Name {
				*(*string)(unsafe.Add(ptr, field.offset)) = cookie.Value
				matched = true
			}
		}
		// 没声明的键值进兜底，没有就直接丢弃
		if !matched && rest != nil {
			if *rest == nil {
				*rest = make(map[string]string, 4)
			}
			(*rest)[cookie.Name] = cookie.Value
		}
	}
	return nil
}

// GetString 从结构体中获取 Cookie 切片并以字符串格式返回
func GetString(v any) (string, error) {
	cookies, err := Get(v)
	if err != nil {
		return "", err
	}
	req := &http.Request{Header: make(http.Header)}
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	return req.Header.Get("Cookie"), nil
}
