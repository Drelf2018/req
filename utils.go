package req

import (
	"net/http"
	"net/url"
	"reflect"
)

// SetClient 设置默认客户端
func SetClient(c *http.Client) func(*Session) error {
	return func(s *Session) error {
		s.Client = *c
		return nil
	}
}

// ParseURL 解析基础路径
func ParseURL(rawURL string) func(*Session) error {
	return func(s *Session) (err error) {
		s.BaseURL, err = url.Parse(rawURL)
		return
	}
}

// SetHeaders 覆盖默认请求头
func SetHeaders(h http.Header) func(*Session) error {
	return func(s *Session) error {
		s.Header = h
		return nil
	}
}

// SetHeaderMap 用映射设置默认请求头
func SetHeaderMap(m map[string]string) func(*Session) error {
	return func(s *Session) error {
		if s.Header == nil {
			s.Header = make(http.Header)
		}
		for k, v := range m {
			s.Header.Set(k, v)
		}
		return nil
	}
}

// SetHeader 设置单个请求头
func SetHeader(k, v string) func(*Session) error {
	return func(s *Session) error {
		if s.Header == nil {
			s.Header = make(http.Header)
		}
		s.Header.Set(k, v)
		return nil
	}
}

// SetAuthorization 设置默认 Authorization 请求头
func SetAuthorization(auth string) func(*Session) error {
	return SetHeader("Authorization", auth)
}

// SetUserAgent 设置默认 User-Agent 请求头
func SetUserAgent(ua string) func(*Session) error {
	return SetHeader("User-Agent", ua)
}

// ToInterface 取 API 实现的接口，没有就返回 nil
func ToInterface[T any](v any) (zero T) {
	i, ok := v.(T)
	if !ok {
		return
	}
	typ := reflect.TypeOf((*T)(nil)).Elem()
	r := reflect.ValueOf(v)
	for r.Kind() == reflect.Pointer || r.Kind() == reflect.Interface {
		if r.IsNil() {
			return
		}
		r = r.Elem()
	}
	if r.Kind() == reflect.Struct {
		t := r.Type()
		for idx := 0; idx < r.NumField(); idx++ {
			sf := t.Field(idx)
			if !sf.Anonymous {
				continue
			}
			f := r.Field(idx)
			if (f.Kind() == reflect.Pointer || f.Kind() == reflect.Interface) && f.IsNil() && sf.Type.Implements(typ) {
				return
			}
		}
	}
	return i
}
