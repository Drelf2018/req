package method

import (
	"bytes"
	"context"
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
type PostJSON struct{}

func (PostJSON) Method() string {
	return http.MethodPost
}

func (PostJSON) Body(ctx context.Context, value reflect.Value, body []reflect.StructField) (io.Reader, error) {
	buf := &bytes.Buffer{}
	err := json.NewEncoder(buf).Encode(MakeJSONMap(ctx, value, body))
	if err != nil {
		return nil, err
	}
	buf.Truncate(buf.Len() - 1) // Seeing the source code of (*json.Encoder).Encode
	return buf, nil
}

var _ APIBody = PostJSON{}

// 以 Form 表单为请求体的 POST 请求构造器
type PostForm struct{}

func (PostForm) Method() string {
	return http.MethodPost
}

func (PostForm) Body(ctx context.Context, value reflect.Value, body []reflect.StructField) (io.Reader, error) {
	return strings.NewReader(MakeURLValues(ctx, value, body).Encode()), nil
}

func (PostForm) Header(r *http.Request, value reflect.Value, header []reflect.StructField) error {
	AddHeader(r, value, header)
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return nil
}

var _ APIBody = PostForm{}
var _ APIHeader = PostForm{}
