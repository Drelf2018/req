package req

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
type PostJSON struct{}

func (PostJSON) Method() string {
	return http.MethodPost
}

func (PostJSON) Body(cli *Client, body []Field, value reflect.Value, api API) (io.Reader, error) {
	m, err := cli.MakeJSONMap(body, value)
	if err != nil {
		return nil, err
	}
	buf := &bytes.Buffer{}
	err = json.NewEncoder(buf).Encode(m)
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

func (PostForm) Body(cli *Client, body []Field, value reflect.Value, api API) (r io.Reader, err error) {
	form, err := cli.MakeURLValues(body, value)
	if err != nil {
		return
	}
	return strings.NewReader(form.Encode()), nil
}

func (PostForm) Header(req *http.Request, cli *Client, header []Field, value reflect.Value, api API) (err error) {
	err = cli.AddHeader(req, header, value)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return
}

var _ APIBody = PostForm{}
var _ APIHeader = PostForm{}
