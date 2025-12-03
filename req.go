package req

import (
	"context"
	"net/http"
	"os"

	"github.com/Drelf2018/req/method"
)

type (
	Get               = method.Get
	PostJSON          = method.PostJSON
	PostForm          = method.PostForm
	PostMultipartForm = method.PostMultipartForm
)

const UserAgent string = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/116.0.0.0 Safari/537.36 Edg/116.0.1938.54"

var DefaultSession = &Session{
	Header: http.Header{
		"User-Agent": {UserAgent},
	},
}

// 发送带上下文的请求
func DoWithContext(ctx context.Context, api API) (*http.Response, error) {
	return DefaultSession.DoWithContext(ctx, api)
}

// 发送请求
func Do(api API) (*http.Response, error) {
	return DefaultSession.Do(api)
}

// 获取带上下文的请求结果
func ContentWithContext(ctx context.Context, api API) ([]byte, error) {
	return DefaultSession.ContentWithContext(ctx, api)
}

// 获取请求结果
func Content(api API) ([]byte, error) {
	return DefaultSession.Content(api)
}

// 获取带上下文的请求结果字符串
func TextWithContext(ctx context.Context, api API) (string, error) {
	return DefaultSession.TextWithContext(ctx, api)
}

// 获取请求结果字符串
func Text(api API) (string, error) {
	return DefaultSession.Text(api)
}

// 将带上下文的请求结果写入文件
func WriteWithContext(ctx context.Context, api API, name string, perm os.FileMode) error {
	return DefaultSession.WriteWithContext(ctx, api, name, perm)
}

// 将请求结果写入文件
func Write(api API, name string, perm os.FileMode) error {
	return DefaultSession.Write(api, name, perm)
}

// 将带上下文的请求结果以 JSON 格式解析进对象
func ResultWithContext[T any](ctx context.Context, api API) (result T, err error) {
	err = DefaultSession.ResultWithContext(ctx, api, &result)
	return
}

// 将请求结果以 JSON 格式解析进对象
func Result[T any](api API) (result T, err error) {
	err = DefaultSession.Result(api, &result)
	return
}

// 将带上下文的请求结果以 JSON 格式解析进接口
func JSONWithContext(ctx context.Context, api API) (any, error) {
	return DefaultSession.JSONWithContext(ctx, api)
}

// 将请求结果以 JSON 格式解析进接口
func JSON(api API) (any, error) {
	return DefaultSession.JSON(api)
}
