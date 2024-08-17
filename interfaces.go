package req

import (
	"context"
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
}

func (PostForm) UseFormBody() {}

type postForm interface {
	UseFormBody()
}

var _ postForm = (*PostForm)(nil)

type PostJson struct {
	Post
}

func (PostJson) UseJsonBody() {}

type postJson interface {
	UseJsonBody()
}

var _ postJson = (*PostJson)(nil)

type Adder interface {
	Add(string, string)
}

var _ Adder = (*url.Values)(nil)
var _ Adder = (*http.Header)(nil)

type NamedReader interface {
	io.Reader
	Name() (filename string)
}

var _ NamedReader = (*os.File)(nil)

type Unwrap interface {
	Unwrap() error
}

type BeforeRequest interface {
	BeforeRequest(context.Context, *Client) error
}
