package req

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

// API 信息
type APIData interface {
	RawURL() string
	Method() string
}

// API 构造器
type APICreator interface {
	NewRequestWithContext(ctx context.Context, cli *Client, api APIData) (*http.Request, error)
}

// API 接口
type API interface {
	APIData
	APICreator
}

// 可添加接口
type Adder interface {
	Add(string, string)
}

var _ Adder = (*url.Values)(nil)
var _ Adder = (*http.Header)(nil)

// 命名的读取器
type NamedReader interface {
	io.Reader
	Name() (filename string)
}

var _ NamedReader = (*os.File)(nil)

// 可解包出错误的接口返回值
type Unwrap interface {
	Unwrap() error
}

// 可判断有效性的 CookieJar
type CookieJar interface {
	IsValid() bool
	http.CookieJar
}

// 重试计时器
type RetryTimer interface {
	// 下次重试前需要等待的时间
	//
	// 参数 num 代表已重试次数 从 0 开始
	//
	// 返回值 time.Duration 代表需要等待的时间
	// 返回值 bool 代表是否继续重试
	NextRetry(num int) (time.Duration, bool)
}
