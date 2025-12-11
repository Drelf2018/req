package cookie

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"sync"
	"unsafe"

	"github.com/Drelf2018/req/method"
)

var offsetsCache sync.Map // map[uintptr]map[string]uintptr

// load 获取对象的底层指针和对应的 Cookie 映射表
func load(v any) (unsafe.Pointer, map[string]uintptr) {
	// 如果已经生成过映射表，直接返回
	e := (*method.Emptyface)(unsafe.Pointer(&v))
	if value, ok := offsetsCache.Load(uintptr(e.Type)); ok {
		return e.Value, value.(map[string]uintptr)
	}
	// 获取对象的类型
	elem := reflect.TypeOf(v)
	if elem.Kind() == reflect.Pointer {
		elem = elem.Elem()
	}
	if elem.Kind() != reflect.Struct {
		return nil, nil
	}
	// 广度优先遍历对象的字段
	offsets := make(map[string]uintptr)
	fields := []reflect.StructField{{Type: elem, Offset: 0}}
	for i := 0; i < len(fields); i++ {
		elem := fields[i]
		numField := elem.Type.NumField()
		for j := 0; j < numField; j++ {
			field := elem.Type.Field(j)
			if !field.IsExported() {
				continue
			}
			// 字符串字段设置了标签则创建映射
			// 结构体字段添加进遍历切片
			if field.Type.Kind() == reflect.String {
				if cookie, ok := field.Tag.Lookup("cookie"); ok {
					offsets[cookie] = elem.Offset + field.Offset
				}
			} else if field.Type.Kind() == reflect.Struct {
				fields = append(fields, reflect.StructField{
					Type:   field.Type,
					Offset: elem.Offset + field.Offset,
				})
			}
		}
	}
	offsetsCache.Store(uintptr(e.Type), offsets)
	return e.Value, offsets
}

// Get 从结构体中获取 Cookie 列表，当传入参数为空、非结构体或其指针时返回错误
func Get(v any) ([]*http.Cookie, error) {
	if v == nil {
		return nil, errors.New("req/cookie.Get: nil value")
	}
	ptr, offsets := load(v)
	if ptr == nil {
		return nil, fmt.Errorf("req/cookie.Get: non-struct type: %T", v)
	}
	cookies := make([]*http.Cookie, 0, len(offsets))
	for name, offset := range offsets {
		cookies = append(cookies, &http.Cookie{Name: name, Value: *(*string)(unsafe.Add(ptr, offset))})
	}
	return cookies, nil
}

// MustGet 从结构体中获取 Cookie 列表，确定传入参数为结构体或其指针时可使用此函数
func MustGet(v any) []*http.Cookie {
	cookies, err := Get(v)
	if err != nil {
		panic(err)
	}
	return cookies
}

// Set 将 Cookie 列表设置到结构体中，当传入参数为空、非结构体或其指针时返回错误
func Set(v any, cookies []*http.Cookie) error {
	if v == nil {
		return errors.New("req/cookie.Set: nil value")
	}
	ptr, offsets := load(v)
	if ptr == nil {
		return fmt.Errorf("req/cookie.Set: non-struct type: %T", v)
	}
	for _, cookie := range cookies {
		if offset, exists := offsets[cookie.Name]; exists {
			*(*string)(unsafe.Add(ptr, offset)) = cookie.Value
		}
	}
	return nil
}

// GetJSON 从结构体中获取 Cookie 列表并以 JSON 格式返回
func GetJSON(v any) ([]byte, error) {
	if v == nil {
		return nil, errors.New("req/cookie.GetJSON: nil value")
	}
	ptr, offsets := load(v)
	if ptr == nil {
		return nil, fmt.Errorf("req/cookie.GetJSON: non-struct type: %T", v)
	}
	m := make(map[string]string, len(offsets))
	for name, offset := range offsets {
		m[name] = *(*string)(unsafe.Add(ptr, offset))
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("req/cookie.GetJSON: %w", err)
	}
	return b, nil
}

// SetJSON 将 JSON 格式的 Cookie 列表设置到结构体中
func SetJSON(v any, data []byte) error {
	m := make(map[string]string)
	err := json.Unmarshal(data, &m)
	if err != nil {
		return fmt.Errorf("req/cookie.SetJSON: %w", err)
	}
	cookies := make([]*http.Cookie, 0, len(m))
	for name, value := range m {
		cookies = append(cookies, &http.Cookie{Name: name, Value: value})
	}
	return Set(v, cookies)
}

// GetString 从结构体中获取 Cookie 列表并以字符串格式返回
func GetString(v any) string {
	req := &http.Request{Header: make(http.Header)}
	for _, cookie := range MustGet(v) {
		req.AddCookie(cookie)
	}
	return req.Header.Get("Cookie")
}
