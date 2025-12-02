package method

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"
)

// GET 请求构造器
//
// 直接嵌入结构体即可使用
type Get struct{}

func (Get) Method() string {
	return http.MethodGet
}

// 以 JSON 为请求体的 POST 请求构造器
type PostJSON struct {
	ContentType string `req:"header" default:"application/json"`
}

func (PostJSON) Method() string {
	return http.MethodPost
}

func (PostJSON) Body(req *http.Request, value reflect.Value, body []reflect.StructField) (io.Reader, error) {
	buf := &bytes.Buffer{}
	err := json.NewEncoder(buf).Encode(MakeJSONMap(req.Context(), value, body))
	if err != nil {
		return nil, err
	}
	buf.Truncate(buf.Len() - 1) // Seeing the source code of (*json.Encoder).Encode
	return buf, nil
}

var _ APIBody = PostJSON{}

// 以 Form 表单为请求体的 POST 请求构造器
type PostForm struct {
	ContentType string `req:"header" default:"application/x-www-form-urlencoded"`
}

func (PostForm) Method() string {
	return http.MethodPost
}

func (PostForm) Body(req *http.Request, value reflect.Value, body []reflect.StructField) (io.Reader, error) {
	return strings.NewReader(MakeURLValues(req.Context(), value, body).Encode()), nil
}

var _ APIBody = PostForm{}
