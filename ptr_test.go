package req_test

import (
	"reflect"
	"testing"

	"github.com/Drelf2018/req"
)

func TestPtr(t *testing.T) {
	ptr1 := req.TypePtr(user{})
	ptr2 := req.ValuePtr(reflect.TypeFor[user]())
	if ptr1 != ptr2 {
		t.Fatal(ptr1, "!=", ptr2)
	}
}

var api req.Api = sendDanmaku{}
var val = reflect.ValueOf(sendDanmaku{})

func BenchmarkApi(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = req.TypePtr(api)
	}
}

func BenchmarkVal(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = req.ValuePtr(val.Type())
	}
}
