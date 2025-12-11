package method

import (
	"io"
	"net/http"
	"reflect"
)

// Marshaler 用于将类型转换为字符串
type Marshaler interface {
	MarshalString() string
}

// API Cookie
type APICookie interface {
	Cookie(r *http.Request, value reflect.Value, cookie []reflect.StructField) error
}

// XSRF 请求头
type APIXSRF interface {
	XSRF() (xsrfCookieName, xsrfHeaderName string)
}

// API 请求体
type APIBody interface {
	Body(r *http.Request, value reflect.Value, body []reflect.StructField) (io.Reader, error)
}

// API 请求参数
type APIQuery interface {
	Query(r *http.Request, value reflect.Value, query []reflect.StructField) error
}

// API 请求头
type APIHeader interface {
	Header(r *http.Request, value reflect.Value, header []reflect.StructField) error
}

// API 自定义标签
type APICustom interface {
	Custom(r *http.Request, value reflect.Value, custom map[string][]reflect.StructField) error
}
