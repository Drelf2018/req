package req_test

import (
	"os"
	"reflect"
	"testing"

	"github.com/Drelf2018/req"
)

type sendDanmaku struct {
	req.Post
	Mode     string   `api:"body;1"`
	Msg      string   `api:"body"`
	Roomid   int      `api:"body"`
	Bubble   int      `api:"body;0"`
	Rnd      int      `api:"body"`
	Color    string   `api:"body;16777215"`
	Fontsize int      `api:"body;25"`
	ReplyMid int      `api:"body,omitempty"`
	UID      int      `api:"query" json:"uid"`
	List     []string `api:"body"`
	File     *os.File `api:"files"`
}

func (sendDanmaku) URL() string {
	return "https://httpbin.org/post"
}

var _ req.Api = (*sendDanmaku)(nil)

func TestReplace(t *testing.T) {
	typ := reflect.TypeFor[sendDanmaku]()
	for i := 0; i < typ.NumField(); i++ {
		name := typ.Field(i).Name
		t.Log(name, "->", req.Replace(name))
	}
}
