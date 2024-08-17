package req_test

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/Drelf2018/req"
)

type color string

func (c color) MarshalString() (string, error) {
	i, err := strconv.ParseInt(strings.ReplaceAll(string(c), "#", ""), 16, 64)
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(i, 10), nil
}

var _ req.Marshaler = (*color)(nil)

type sendDanmaku struct {
	req.PostJson
	UID            int      `api:"query"`
	Mode           string   `api:"body;1"`
	Msg            string   `api:"body"`
	Color          color    `api:"body;16777215"`
	ReplyMID       int      `api:"body,omitempty"`
	List           []string `api:"body"`
	File           *os.File `api:"files"`
	AcceptLanguage string   `api:"header"`
}

func (sendDanmaku) URL() string {
	return "https://httpbin.org/post"
}

func (api *sendDanmaku) BeforeRequest(context.Context, *req.Client) (err error) {
	api.File, err = os.Open("go.mod")
	api.Msg = "你好"
	return
}

var _ req.BeforeRequest = (*sendDanmaku)(nil)

func TestDebug(t *testing.T) {
	data, err := req.Debug(&sendDanmaku{UID: 12138, List: []string{"1", "1", "4"}, Color: "#00FFFF", AcceptLanguage: "zh-CN"})
	if err != nil {
		t.Fatal(data, err)
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
	return fmt.Errorf("req_test: %s(%d, %d)", e.Message, e.Code, e.ErrCode)
}

var gateway = TestApi("https://api.sgroup.qq.com/gateway")

type gatewayResponse struct {
	*ErrMessage
	URL string `json:"url"`
}

func TestGateway(t *testing.T) {
	result, err := req.Do[gatewayResponse](gateway)
	if err == nil {
		t.Fatal(result)
	}
	t.Log(err)
}
