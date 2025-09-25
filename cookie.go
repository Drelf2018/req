package req

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"unsafe"
)

type StructCookieJar[T any] struct {
	// Cookie 结构体的底层实现
	//
	// 当字段导出且类型为 string 时才会被用作 Cookie 值，字段名作为 Cookie 名。
	// 可以使用 `cookie` 标签或者 `json` 标签进行重命名，两者都存在时优先使用 `cookie` 标签。
	//
	//	var jar req.StructCookieJar[struct {
	//		UID      string `json:"uid"`
	//		Username string `cookie:"username"`
	//	}]
	C       T
	rw      sync.RWMutex
	ptr     unsafe.Pointer
	offsets map[string]uintptr
}

// 计算所有可作为 Cookie 字段的偏移量
func (s *StructCookieJar[T]) init(t reflect.Type, parentOffset uintptr) {
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		if field.Type.Kind() == reflect.String {
			name, exists := field.Tag.Lookup("cookie")
			if !exists {
				name, exists = field.Tag.Lookup("json")
				if exists {
					name = strings.Split(name, ",")[0]
				}
			} else if name == "-" {
				continue
			}
			if !exists {
				name = field.Name
			}
			s.offsets[name] = parentOffset + field.Offset
		} else if field.Type.Kind() == reflect.Struct {
			s.init(field.Type, parentOffset+field.Offset)
		}
	}
}

// 设置在结构体中存在的 Cookie 字段，不会判断是否与 u 同源
func (s *StructCookieJar[T]) SetCookies(u *url.URL, cookies []*http.Cookie) {
	s.rw.Lock()
	defer s.rw.Unlock()
	if s.offsets == nil {
		s.ptr = unsafe.Pointer(s)
		s.offsets = make(map[string]uintptr)
		s.init(reflect.TypeOf(s.C), unsafe.Offsetof(s.C))
	}
	for _, cookie := range cookies {
		offset, exists := s.offsets[cookie.Name]
		if exists {
			*(*string)(unsafe.Add(s.ptr, offset)) = cookie.Value
		}
	}
}

// 返回所有已设置的 Cookie 切片，不会判断是否与 u 同源
func (s *StructCookieJar[T]) Cookies(u *url.URL) []*http.Cookie {
	if s.offsets == nil {
		return nil
	}
	s.rw.RLock()
	defer s.rw.RUnlock()
	cookies := make([]*http.Cookie, 0, len(s.offsets))
	for name, offset := range s.offsets {
		cookies = append(cookies, &http.Cookie{Name: name, Value: *(*string)(unsafe.Add(s.ptr, offset))})
	}
	return cookies
}

var _ http.CookieJar = (*StructCookieJar[any])(nil)

// 返回指定名称的 Cookie
func (s *StructCookieJar[T]) Get(name string) (cookie *http.Cookie, exists bool) {
	if s.offsets == nil {
		return nil, false
	}
	s.rw.RLock()
	defer s.rw.RUnlock()
	var offset uintptr
	offset, exists = s.offsets[name]
	if exists {
		cookie = &http.Cookie{Name: name, Value: *(*string)(unsafe.Add(s.ptr, offset))}
	}
	return
}

func (s *StructCookieJar[T]) MarshalJSON() ([]byte, error) {
	if s.offsets == nil {
		return nil, nil
	}
	s.rw.RLock()
	defer s.rw.RUnlock()
	first := true
	buf := bytes.NewBuffer([]byte{'{'})
	for name, offset := range s.offsets {
		if !first {
			buf.WriteByte(',')
		}
		first = false
		buf.WriteString(`"` + name + `":"` + *(*string)(unsafe.Add(s.ptr, offset)) + `"`)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

var _ json.Marshaler = (*StructCookieJar[any])(nil)

func (s *StructCookieJar[T]) UnmarshalJSON(data []byte) error {
	m := make(map[string]string)
	err := json.Unmarshal(data, &m)
	if err != nil {
		return fmt.Errorf("req: unmarshal StructCookieJar failed: %w", err)
	}
	cookies := make([]*http.Cookie, 0, len(m))
	for name, value := range m {
		cookies = append(cookies, &http.Cookie{Name: name, Value: value})
	}
	s.SetCookies(nil, cookies)
	return nil
}

var _ json.Unmarshaler = (*StructCookieJar[any])(nil)
