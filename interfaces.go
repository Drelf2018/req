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
	Method() string
	RawURL() string
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

type FuncAdder func(string, string)

func (f FuncAdder) Add(key, val string) {
	f(key, val)
}

var _ Adder = FuncAdder(nil)

// 重试计时器
type RetryTicker interface {
	// 下次重试前需要等待的时间
	//
	// 参数 retried 表示从 0 开始的已重试次数
	//
	// 返回值 delay 表示需要等待的时间
	//
	// 返回值 ok 表示是否继续重试
	NextRetry(retried int) (delay time.Duration, ok bool)
}

// 请求前钩子
type BeforeRequest interface {
	BeforeRequest(req *http.Request, cli *http.Client, api API, retried int)
}

// 检验响应
type CheckResponse interface {
	CheckResponse(resp *http.Response, cli *http.Client, api API, retried int) error
}

// 可解包出错误的接口返回值
type Unwrap interface {
	Unwrap() error
}
