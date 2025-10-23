package cookie

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"sync"
	"unsafe"

	"github.com/Drelf2018/req"
)

var offsetsCache sync.Map // map[uintptr]map[string]uintptr

func parse(t reflect.Type, parentOffset uintptr, offsets map[string]uintptr) {
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		if field.Type.Kind() == reflect.String {
			name := field.Tag.Get("cookie")
			if name == "-" {
				continue
			}
			if name == "" {
				name = field.Name
			}
			offsets[name] = parentOffset + field.Offset
		} else if field.Type.Kind() == reflect.Struct {
			parse(field.Type, parentOffset+field.Offset, offsets)
		}
	}
}

func load(v any) (unsafe.Pointer, map[string]uintptr) {
	e := (*req.Any)(unsafe.Pointer(&v))
	if value, ok := offsetsCache.Load(uintptr(e.Type)); ok {
		return e.Value, value.(map[string]uintptr)
	}
	elem := reflect.TypeOf(v)
	if elem.Kind() == reflect.Pointer {
		elem = elem.Elem()
	}
	if elem.Kind() != reflect.Struct {
		return nil, nil
	}
	offsets := make(map[string]uintptr)
	parse(elem, 0, offsets)
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
