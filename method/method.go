package method

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
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

// 以多部份 Form 表单为请求体的 POST 请求构造器
type PostMultipartForm struct {
	ContentType string `req:"header" default:"$ContentType"`
}

func (PostMultipartForm) Method() string {
	return http.MethodPost
}

func (PostMultipartForm) Body(req *http.Request, value reflect.Value, body []reflect.StructField) (io.Reader, error) {
	ctx := req.Context()
	buf := &bytes.Buffer{}
	multi := multipart.NewWriter(buf)
	for _, field := range body {
		val, err := value.FieldByIndexErr(field.Index)
		if err != nil {
			return nil, err
		}
		switch v := val.Interface().(type) {
		case io.Reader:
			if val.IsZero() {
				if len(field.Tag) == 0 {
					continue
				}
				return nil, fmt.Errorf("req/method: invalid file %q", field.Name)
			}
			name := field.Name
			if namer, ok := v.(interface{ Name() (filename string) }); ok {
				name = namer.Name()
			}
			writer, err := multi.CreateFormFile(field.Name, name)
			if err != nil {
				return nil, err
			}
			_, err = io.Copy(writer, v)
			if err != nil {
				return nil, err
			}
			if closer, ok := v.(io.Closer); ok {
				err = closer.Close()
			}
			if err != nil {
				return nil, err
			}
		default:
			AddValue(ctx, value, field, func(key, val string) { multi.WriteField(key, val) })
		}
	}
	err := multi.Close()
	if err != nil {
		return nil, err
	}
	ctx.(interface{ Set(string, any) }).Set("$ContentType", multi.FormDataContentType())
	return buf, nil
}

var _ APIBody = PostMultipartForm{}
