package req

import (
	"io"
	"net/http"
	"net/url"
	"os"
)

type Api interface {
	URL() string
	Method() string
}

type Get struct{}

func (Get) Method() string {
	return http.MethodGet
}

type Post struct{}

func (Post) Method() string {
	return http.MethodPost
}

type PostForm struct {
	Post
	ContentType string `api:"header;application/x-www-form-urlencoded" json:"Content-Type"`
}

type PostJson struct {
	Post
	ContentType string `api:"header;application/json" json:"Content-Type"`
}

func (PostJson) JsonBody() {}

type postJson interface {
	JsonBody()
}

var _ postJson = (*PostJson)(nil)

type JsonBody map[string]string

func (m JsonBody) Add(key, value string) {
	m[key] = value
}

type Adder interface {
	Add(string, string)
}

var _ Adder = (*JsonBody)(nil)
var _ Adder = (*url.Values)(nil)
var _ Adder = (*http.Header)(nil)

type NamedReader interface {
	io.Reader
	Name() (filename string)
}

var _ NamedReader = (*os.File)(nil)
