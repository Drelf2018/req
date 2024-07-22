package req_test

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/Drelf2018/req"
)

var bodyData = url.Values{
	"bubble":   {"0"},
	"color":    {"16777215"},
	"fontsize": {"25"},
	"mode":     {"1"},
	"msg":      {""},
	"rnd":      {""},
	"roomid":   {""},
}

func BenchmarkHttp(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = http.PostForm("https://httpbin.org/post", bodyData)
	}
}

func BenchmarkDebug(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = req.Debug(sendDanmaku{})
	}
}

// goos: windows
// goarch: amd64
// pkg: github.com/Drelf2018/req
// cpu: Intel(R) Core(TM) i7-1065G7 CPU @ 1.30GHz
// BenchmarkHttp-8    	       4	 251906625 ns/op	    7260 B/op	      68 allocs/op
// BenchmarkDebug-8   	       4	 435558275 ns/op	   12470 B/op	     137 allocs/op
