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

// 发送带上下文的请求
func DoWithContext(ctx context.Context, api API) (*http.Response, error) {
	return DefaultClient.DoWithContext(ctx, api)
}

// 发送请求
func Do(api API) (*http.Response, error) {
	return DefaultClient.Do(api)
}

// 获取带上下文的请求结果
func ContentWithContext(ctx context.Context, api API) ([]byte, error) {
	return DefaultClient.ContentWithContext(ctx, api)
}

// 获取请求结果
func Content(api API) ([]byte, error) {
	return DefaultClient.Content(api)
}

// 获取带上下文的请求结果字符串
func TextWithContext(ctx context.Context, api API) (string, error) {
	return DefaultClient.TextWithContext(ctx, api)
}

// 获取请求结果字符串
func Text(api API) (string, error) {
	return DefaultClient.Text(api)
}

// 将带上下文的请求结果写入文件
func WriteWithContext(ctx context.Context, api API, name string, perm os.FileMode) error {
	return DefaultClient.WriteWithContext(ctx, api, name, perm)
}

// 将请求结果写入文件
func Write(api API, name string, perm os.FileMode) error {
	return DefaultClient.Write(api, name, perm)
}

// 将带上下文的请求结果以 JSON 格式解析进对象
func ResultWithContext[T any](ctx context.Context, api API) (result T, err error) {
	err = DefaultClient.ResultWithContext(ctx, api, &result)
	return
}

// 将请求结果以 JSON 格式解析进对象
func Result[T any](api API) (result T, err error) {
	err = DefaultClient.Result(api, &result)
	return
}

// 将带上下文的请求结果以 JSON 格式解析进接口
func JSONWithContext(ctx context.Context, api API) (any, error) {
	return DefaultClient.JSONWithContext(ctx, api)
}

// 将请求结果以 JSON 格式解析进接口
func JSON(api API) (any, error) {
	return DefaultClient.JSON(api)
}

// 生成 cURL
func CURL(api API) (string, error) {
	return DefaultClient.CURL(api)
}

// 将请求结果改写成结构体
func Struct(api API, name string) ([]byte, error) {
	return DefaultClient.Struct(api, name)
}

// 在指定文件写入请求结果改写的结构体
func Generate(filename string, api API) error {
	return DefaultClient.Generate(filename, api)
}

// 克隆客户端
func Clone(rawURL string) (*Client, error) {
	return DefaultClient.Clone(rawURL)
}

// 必须克隆！
func MustClone(rawURL string) *Client {
	return DefaultClient.MustClone(rawURL)
}
