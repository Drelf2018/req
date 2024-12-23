package req

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"os"
)

type APIData interface {
	RawURL() string
	Method() string
}

type APICreator interface {
	NewRequestWithContext(ctx context.Context, cli *Client, api APIData) (*http.Request, error)
}

type API interface {
	APIData
	APICreator
}

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

type CookieJar interface {
	IsValid() bool
	http.CookieJar
}
