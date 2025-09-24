package req

import (
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
	C            T
	rw           sync.RWMutex
	ptr          unsafe.Pointer
	fieldOffsets map[string]uintptr
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
			}
			if !exists {
				name = field.Name
			}
			s.fieldOffsets[name] = parentOffset + field.Offset
		} else if field.Type.Kind() == reflect.Struct {
			s.init(field.Type, parentOffset+field.Offset)
		}
	}
}

// 设置在结构体中存在的 Cookie 字段，不会判断是否与 u 同源
func (s *StructCookieJar[T]) SetCookies(u *url.URL, cookies []*http.Cookie) {
	s.rw.Lock()
	defer s.rw.Unlock()
	if s.fieldOffsets == nil {
		s.ptr = unsafe.Pointer(s)
		s.fieldOffsets = make(map[string]uintptr)
		s.init(reflect.TypeOf(s.C), unsafe.Offsetof(s.C))
	}
	for _, cookie := range cookies {
		offset, exists := s.fieldOffsets[cookie.Name]
		if exists {
			*(*string)(unsafe.Add(s.ptr, offset)) = cookie.Value
		}
	}
}

// 返回所有已设置的 Cookie 切片，不会判断是否与 u 同源
func (s *StructCookieJar[T]) Cookies(u *url.URL) []*http.Cookie {
	if s.fieldOffsets == nil {
		return nil
	}
	s.rw.RLock()
	defer s.rw.RUnlock()
	cookies := make([]*http.Cookie, 0, len(s.fieldOffsets))
	for name, offset := range s.fieldOffsets {
		cookies = append(cookies, &http.Cookie{Name: name, Value: *(*string)(unsafe.Add(s.ptr, offset))})
	}
	return cookies
}

// 返回指定名称的 Cookie
func (s *StructCookieJar[T]) Get(name string) (cookie *http.Cookie, exists bool) {
	if s.fieldOffsets == nil {
		return nil, false
	}
	s.rw.RLock()
	defer s.rw.RUnlock()
	var offset uintptr
	offset, exists = s.fieldOffsets[name]
	if exists {
		cookie = &http.Cookie{Name: name, Value: *(*string)(unsafe.Add(s.ptr, offset))}
	}
	return
}

var _ http.CookieJar = (*StructCookieJar[any])(nil)
