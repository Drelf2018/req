package req_test

import (
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
	req.PostJSON
	UID            int      `api:"query"`
	Mode           string   `api:"body:1"`
	Msg            string   `api:"body"`
	Color          color    `api:"body:16777215"`
	ReplyMID       int      `api:"body,omitempty"`
	List           []string `api:"body"`
	File           *os.File `api:"files"`
	AcceptLanguage string   `api:"header"`
}

func (sendDanmaku) RawURL() string {
	return "https://httpbin.org/post"
}

func TestDebug(t *testing.T) {
	api := &sendDanmaku{UID: 12138, Msg: "你好", List: []string{"1", "1", "4"}, Color: "#00FFFF", AcceptLanguage: "zh-CN"}
	var err error
	api.File, err = os.Open("go.mod")
	if err != nil {
		t.Fatal(err)
	}
	data, err := req.JSON(api)
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

var gateway = MinAPI("https://api.sgroup.qq.com/gateway")

type gatewayResponse struct {
	*ErrMessage
	URL string `json:"url"`
}

func TestGateway(t *testing.T) {
	result, err := req.Result[gatewayResponse](gateway)
	if err == nil {
		t.Fatal(result)
	}
	t.Log(err)
}
