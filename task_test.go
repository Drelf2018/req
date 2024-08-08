package req_test

import "testing"

func URL() string {
	return "/get"
}

var baseURL1 = ""
var baseURL2 = "https://httpbin.org"

func BenchmarkAdd1(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = baseURL1 + URL()
	}
}

func BenchmarkAdd2(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = baseURL2 + URL()
	}
}

func BenchmarkIf1(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if baseURL1 != "" {
			_ = baseURL1 + URL()
		} else {
			_ = api.URL()
		}
	}
}
func BenchmarkIf2(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if baseURL2 != "" {
			_ = baseURL2 + URL()
		} else {
			_ = api.URL()
		}
	}
}
