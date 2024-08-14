package req_test

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/Drelf2018/req"
)

type Color string

func (c Color) MarshalString() (string, error) {
	i, err := strconv.ParseInt(strings.ReplaceAll(string(c), "#", ""), 16, 64)
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(i, 10), nil
}

var _ req.Marshaler = (*Color)(nil)

type sendDanmaku struct {
	req.PostJson
	UID            int      `api:"query"`
	Mode           string   `api:"body;1"`
	Msg            string   `api:"body"`
	Color          Color    `api:"body;16777215"`
	ReplyMID       int      `api:"body,omitempty"`
	List           []string `api:"body"`
	File           *os.File `api:"files"`
	AcceptLanguage string   `api:"header"`
}

func (sendDanmaku) URL() string {
	return "https://httpbin.org/post"
}

func TestDebug(t *testing.T) {
	api := sendDanmaku{UID: 12138, List: []string{"1", "1", "4"}, Color: "#FFFFFF", AcceptLanguage: "zh-CN"}
	api.File, _ = os.Open("go.mod")
	data, err := req.Debug(api)
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
	return fmt.Errorf("dto: %s(%d, %d)", e.Message, e.Code, e.ErrCode)
}

var gateway = req.GetURL[struct {
	*ErrMessage
	URL string `json:"url"`
}]("https://api.sgroup.qq.com/gateway")

func TestGateway(t *testing.T) {
	data, err := gateway.Do()
	if err == nil {
		t.Fatal(data)
	}
	t.Logf("%#v, %#v", data, data.ErrMessage)
}
