package req

import (
	"bytes"
	"context"
	"io"
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

// NamedReader 命名读取器
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

// UserAgent 默认 UA 请求头
const UserAgent string = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/116.0.0.0 Safari/537.36 Edg/116.0.1938.54"

// DefaultSession 默认会话
var DefaultSession = &Session{
	Header: http.Header{
		"User-Agent": {UserAgent},
	},
}

// NewRequestWithContext 携带上下文新建请求
func NewRequestWithContext(ctx context.Context, api API) (*http.Request, error) {
	return DefaultSession.NewRequestWithContext(ctx, api)
}

// NewRequest 新建请求
func NewRequest(api API) (*http.Request, error) {
	return DefaultSession.NewRequest(api)
}

// DoWithContext 携带上下文发送请求
func DoWithContext(ctx context.Context, api API) (*http.Response, error) {
	return DefaultSession.DoWithContext(ctx, api)
}

// Do 发送请求
func Do(api API) (*http.Response, error) {
	return DefaultSession.Do(api)
}

// ContentWithContext 携带上下文获取请求结果
func ContentWithContext(ctx context.Context, api API) ([]byte, error) {
	return DefaultSession.ContentWithContext(ctx, api)
}

// Content 获取请求结果
func Content(api API) ([]byte, error) {
	return DefaultSession.Content(api)
}

// TextWithContext 携带上下文获取请求结果字符串
func TextWithContext(ctx context.Context, api API) (string, error) {
	return DefaultSession.TextWithContext(ctx, api)
}

// Text 获取请求结果字符串
func Text(api API) (string, error) {
	return DefaultSession.Text(api)
}

// WriteWithContext 携带上下文将请求结果写入文件
func WriteWithContext(ctx context.Context, api API, name string, perm os.FileMode) error {
	return DefaultSession.WriteWithContext(ctx, api, name, perm)
}

// Write 将请求结果写入文件
func Write(api API, name string, perm os.FileMode) error {
	return DefaultSession.Write(api, name, perm)
}

// ResultWithContext 携带上下文将请求结果以 JSON 格式反序列化进对象
func ResultWithContext[T any](ctx context.Context, api API) (result T, err error) {
	err = DefaultSession.ResultWithContext(ctx, api, &result)
	return
}

// Result 将请求结果以 JSON 格式反序列化进对象
func Result[T any](api API) (result T, err error) {
	err = DefaultSession.Result(api, &result)
	return
}

// JSONWithContext 携带上下文将请求结果以 JSON 格式反序列化进接口
func JSONWithContext(ctx context.Context, api API) (any, error) {
	return DefaultSession.JSONWithContext(ctx, api)
}

// JSON 将请求结果以 JSON 格式反序列化进接口
func JSON(api API) (any, error) {
	return DefaultSession.JSON(api)
}
