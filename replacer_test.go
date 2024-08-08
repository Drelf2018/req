package req_test

import (
	"reflect"
	"testing"

	"github.com/Drelf2018/req"
)

func TestReplace(t *testing.T) {
	typ := reflect.TypeFor[sendDanmaku]()
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		name := field.Name
		if field.Tag.Get("api") == "header" {
			t.Log(name, "->", req.HeaderReplace(name))
		} else {
			t.Log(name, "->", req.KeyReplace(name))
		}
	}
}
