package req

import (
	"io"
	"net/http"
	"net/url"
	"reflect"
	"time"
)

// API 接口
type API interface {
	RawURL() string
	Method() string
}

// API 请求体
type APIBody interface {
	Body(cli *Client, body []Field, value reflect.Value, api API) (io.Reader, error)
}

// API 请求参数
type APIQuery interface {
	Query(req *http.Request, cli *Client, query []Field, value reflect.Value, api API) error
}

// API 请求头
type APIHeader interface {
	Header(req *http.Request, cli *Client, header []Field, value reflect.Value, api API) error
}

// 可添加接口
type Adder interface {
	Add(string, string)
}

var _ Adder = (*url.Values)(nil)
var _ Adder = (*http.Header)(nil)
var _ Adder = (*CookieAdder)(nil)

// 检验响应
type CheckResponse interface {
	CheckResponse(*http.Response) error
}

// 重试计时器
type RetryTicker interface {
	// 下次重试前需要等待的时间
	//
	// 参数 num 代表已重试次数 从 0 开始
	//
	// 返回值 time.Duration 代表需要等待的时间
	//
	// 返回值 bool 代表是否继续重试
	NextRetry(num int) (time.Duration, bool)
}

// 可解包出错误的接口返回值
type Unwrap interface {
	Unwrap() error
}
