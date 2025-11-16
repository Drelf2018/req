package req

import (
	"net/http"
	"time"
)

// API 接口
type API interface {
	Method() string
	RawURL() string
}

// 请求前钩子
type BeforeRequest interface {
	BeforeRequest(cli *http.Client, req *http.Request, api API) error
}

// 检验响应
type CheckResponse interface {
	CheckResponse(cli *http.Client, resp *http.Response, api API) error
}

// 可解包出错误的接口返回值
type Unwrap interface {
	Unwrap() error
}

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
