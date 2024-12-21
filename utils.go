package req

import (
	"context"
	"net/http"
	"os"
)

const UserAgent string = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/116.0.0.0 Safari/537.36 Edg/116.0.1938.54"

var DefaultClient = &Client{
	Header: http.Header{
		"User-Agent": {UserAgent},
	},
}

func DoWithContext(ctx context.Context, api API) (*http.Response, error) {
	return DefaultClient.DoWithContext(ctx, api)
}

func Do(api API) (*http.Response, error) {
	return DefaultClient.Do(api)
}

func ContentWithContext(ctx context.Context, api API) ([]byte, error) {
	return DefaultClient.ContentWithContext(ctx, api)
}

func Content(api API) ([]byte, error) {
	return DefaultClient.Content(api)
}

func TextWithContext(ctx context.Context, api API) (string, error) {
	return DefaultClient.TextWithContext(ctx, api)
}

func Text(api API) (string, error) {
	return DefaultClient.Text(api)
}

func WriteWithContext(ctx context.Context, api API, name string, perm os.FileMode) error {
	return DefaultClient.WriteWithContext(ctx, api, name, perm)
}

func Write(api API, name string, perm os.FileMode) error {
	return DefaultClient.Write(api, name, perm)
}

func ResultWithContext[T any](ctx context.Context, api API) (result T, err error) {
	err = DefaultClient.ResultWithContext(ctx, api, &result)
	return
}

func Result[T any](api API) (result T, err error) {
	err = DefaultClient.Result(api, &result)
	return
}

func JSONWithContext(ctx context.Context, api API) (any, error) {
	return DefaultClient.JSONWithContext(ctx, api)
}

func JSON(api API) (any, error) {
	return DefaultClient.JSON(api)
}

func CURL(api API) (string, error) {
	return DefaultClient.CURL(api)
}

func Struct(api API, name string) ([]byte, error) {
	return DefaultClient.Struct(api, name)
}

func Generate(filename string, api API) error {
	return DefaultClient.Generate(filename, api)
}

func Clone(rawURL string) (*Client, error) {
	return DefaultClient.Clone(rawURL)
}

func MustClone(rawURL string) *Client {
	return DefaultClient.MustClone(rawURL)
}
