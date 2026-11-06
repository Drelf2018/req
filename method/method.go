package method

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"reflect"
	"strings"
)

// Get 是 GET 请求构造器，直接嵌入结构体使用
type Get struct{}

func (Get) Method() string {
	return http.MethodGet
}

// PostJSON 是 POST 请求构造器，以 JSON 格式创建请求体
type PostJSON struct{}

func (PostJSON) Method() string {
	return http.MethodPost
}

func (PostJSON) Body(ctx context.Context, api API, reader CookieReader, body []Field) (io.Reader, http.Header, error) {
	m, err := MakeMap(ctx, api, reader, body)
	if err != nil {
		return nil, nil, err
	}
	r, err := NewJSONReader(m)
	if err != nil {
		return nil, nil, err
	}
	return r, ContentType("application/json"), nil
}

var _ APIBody = PostJSON{}

// PostForm 是 POST 请求构造器，以 Form 表单创建请求体
type PostForm struct{}

func (PostForm) Method() string {
	return http.MethodPost
}

func (PostForm) Body(ctx context.Context, api API, reader CookieReader, body []Field) (io.Reader, http.Header, error) {
	form, err := MakeURLValues(ctx, api, reader, body)
	if err != nil {
		return nil, nil, err
	}
	return strings.NewReader(form.Encode()), ContentType("application/x-www-form-urlencoded"), nil
}

var _ APIBody = PostForm{}

// NamedReader 是拥有名字的字符读取器
type NamedReader struct {
	name   string
	reader *bytes.Reader
}

func (n *NamedReader) Name() string {
	return n.name
}

func (n *NamedReader) Read(p []byte) (int, error) {
	return n.reader.Read(p)
}

var _ io.Reader = (*NamedReader)(nil)

// NewNamedReader 新建命名读取器
func NewNamedReader(name string, data []byte) *NamedReader {
	return &NamedReader{name, bytes.NewReader(data)}
}

// PostMultipartForm 是 POST 请求构造器，以多部份 Form 表单创建请求体。
// 文件字段在构建请求体时会被 Close，所以它被重试时会因无法再次读取而报错。
type PostMultipartForm struct{}

func (PostMultipartForm) Method() string {
	return http.MethodPost
}

func (PostMultipartForm) Body(ctx context.Context, api API, reader CookieReader, body []Field) (io.Reader, http.Header, error) {
	// 创建多部份表单和载体
	buf := &bytes.Buffer{}
	multi := multipart.NewWriter(buf)
	// 获取 API 的真实值
	root := reflect.Indirect(reflect.ValueOf(api))
	for _, f := range body {
		// 一般不会有人传入与字段索引不对应的值，这里做个保险
		value, err := root.FieldByIndexErr(f.Index)
		if err != nil {
			return nil, nil, fmt.Errorf("req/method: cannot reach field %q: %w", f.Name, err)
		}
		switch v := value.Interface().(type) {
		case io.Reader:
			if value.IsZero() {
				if f.Omit {
					continue
				}
				return nil, nil, fmt.Errorf("req/method: field %q is an invalid file", f.Name)
			}
			name := f.Name
			if named, ok := v.(interface{ Name() string }); ok {
				name = named.Name()
			}
			writer, err := multi.CreateFormFile(f.Name, name)
			if err != nil {
				return nil, nil, fmt.Errorf("req/method: cannot write form file for field %q: %w", f.Name, err)
			}
			_, err = io.Copy(writer, v)
			if err != nil {
				return nil, nil, fmt.Errorf("req/method: cannot copy file content for field %q: %w", f.Name, err)
			}
			if closer, ok := v.(io.Closer); ok {
				err = closer.Close()
			}
			if err != nil {
				return nil, nil, fmt.Errorf("req/method: cannot close file for field %q: %w", f.Name, err)
			}
		default:
			// 普通字段：跑取值链，逐条写进表单（切片会写成多条同名 part）
			ss, err := f.ToString(ctx, root, reader)
			if err != nil {
				return nil, nil, err
			}
			for _, s := range ss {
				if err := multi.WriteField(f.Name, s); err != nil {
					return nil, nil, fmt.Errorf("req/method: cannot write field %q: %w", f.Name, err)
				}
			}
		}
	}
	err := multi.Close()
	if err != nil {
		return nil, nil, fmt.Errorf("req/method: cannot close form: %w", err)
	}
	return buf, ContentType(multi.FormDataContentType()), nil
}

var _ APIBody = PostMultipartForm{}
