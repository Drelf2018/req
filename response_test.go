package req_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/Drelf2018/req"
)

func TestDebug(t *testing.T) {
	api := sendDanmaku{UID: 12138, List: []string{"1", "1", "4"}}
	api.File, _ = os.Open("go.mod")

	data, err := req.Debug(api)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(data)
}

type ErrMessage struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
	ErrCode int    `json:"err_code"`
	TraceID string `json:"trace_id"`
}

func (e *ErrMessage) Unwrap() error {
	if e == nil {
		return nil
	}
	return fmt.Errorf("dto: %s(%d, %d)", e.Message, e.Code, e.ErrCode)
}

type GatewayResponse struct {
	*ErrMessage
	URL string `json:"url"`
}

type Gateway struct {
	req.Get
}

func (Gateway) URL() string {
	// return "https://httpbin.org/get"
	return "https://api.sgroup.qq.com/gateway"
}

func (api Gateway) Do() (GatewayResponse, error) {
	return req.Do[GatewayResponse](api)
}

func TestGateway(t *testing.T) {
	data, err := Gateway{}.Do()
	if err == nil {
		t.Fatal(data)
	}
	t.Logf("%#v, %#v", data, data.ErrMessage)
}
