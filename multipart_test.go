package req_test

import (
	"bytes"
	"mime/multipart"
	"testing"
)

type embededWriter struct {
	*multipart.Writer
}

type aliasWriter multipart.Writer

func (w *aliasWriter) WriteField(fieldname string, value string) error {
	return (*multipart.Writer)(w).WriteField(fieldname, value)
}

func BenchmarkEmbeded(b *testing.B) {
	w := embededWriter{multipart.NewWriter(&bytes.Buffer{})}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = w.WriteField("114", "514")
	}
}

func BenchmarkAlias(b *testing.B) {
	w := (*aliasWriter)(multipart.NewWriter(&bytes.Buffer{}))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = w.WriteField("114", "514")
	}
}

// goos: windows
// goarch: amd64
// pkg: github.com/Drelf2018/req
// cpu: Intel(R) Core(TM) i7-1065G7 CPU @ 1.30GHz
// BenchmarkEmbeded-8   	 1335844	       857.0 ns/op	     849 B/op	      12 allocs/op
// BenchmarkAlias-8     	 1499043	       803.2 ns/op	     806 B/op	      12 allocs/op
