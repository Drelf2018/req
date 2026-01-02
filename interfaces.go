package req

import (
	"net/http"

	"github.com/Drelf2018/req/method"
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

type (
	APICookie = method.APICookie
	APIXSRF   = method.APIXSRF
	APIBody   = method.APIBody
	APIQuery  = method.APIQuery
	APIHeader = method.APIHeader
	APICustom = method.APICustom
)
