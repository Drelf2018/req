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

// BeforeRequest 请求前钩子
type BeforeRequest interface {
	BeforeRequest(cli *http.Client, req *http.Request, api API) error
}

// CheckResponse 检验响应，出现错误时必须自行调用 resp.Body.Close()
type CheckResponse interface {
	CheckResponse(cli *http.Client, resp *http.Response, api API) error
}

// Unwrap 用于从响应体中解包出错误
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
