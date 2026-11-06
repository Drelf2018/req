package template

import (
	"context"
	"io"
	"net/http"
	"testing"
)

func readBody(t *testing.T, req *http.Request) string {
	t.Helper()
	if req.Body == nil {
		return ""
	}
	b, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestArgsToRequest 覆盖各参数对最终请求的影响
func TestArgsToRequest(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name    string
		args    []string
		method  string
		url     string
		body    string
		header  map[string]string
		cookie  string
		wantErr bool
	}{
		{
			name:   "只有 URL，默认 GET",
			args:   []string{"https://httpbin.org/get"},
			method: http.MethodGet,
			url:    "https://httpbin.org/get",
		},
		{
			name:   "-d 默认变 POST",
			args:   []string{"https://httpbin.org/post", "-d", "a=1&b=2"},
			method: http.MethodPost,
			url:    "https://httpbin.org/post",
			body:   "a=1&b=2",
		},
		{
			name:   "多个 -d 用 & 拼接",
			args:   []string{"https://httpbin.org/post", "-d", "a=1", "-d", "b=2"},
			method: http.MethodPost,
			url:    "https://httpbin.org/post",
			body:   "a=1&b=2",
		},
		{
			name:   "-X POST 显式方法",
			args:   []string{"https://httpbin.org/put", "-X", "PUT", "-d", "a=1"},
			method: http.MethodPut,
			url:    "https://httpbin.org/put",
			body:   "a=1",
		},
		{
			name:   "-X 粘连写法",
			args:   []string{"https://httpbin.org/delete", "-XDELETE"},
			method: http.MethodDelete,
			url:    "https://httpbin.org/delete",
		},
		{
			// url.Values.Encode 会按键排序，所以 a、b、x 的顺序是固定的
			name:   "-G 把 -d 拼进 query",
			args:   []string{"https://httpbin.org/get?x=0", "-G", "-d", "a=1&b=2"},
			method: http.MethodGet,
			url:    "https://httpbin.org/get?a=1&b=2&x=0",
		},
		{
			name:   "-G 且只有 -d 也是 GET",
			args:   []string{"https://httpbin.org/get", "-G", "-d", "a=1"},
			method: http.MethodGet,
			url:    "https://httpbin.org/get?a=1",
		},
		{
			name:   "-H 设置请求头",
			args:   []string{"https://httpbin.org/get", "-H", "X-Test: hello"},
			method: http.MethodGet,
			url:    "https://httpbin.org/get",
			header: map[string]string{"X-Test": "hello"},
		},
		{
			name:   "-A 与 -e",
			args:   []string{"https://httpbin.org/get", "-A", "Nana7mi", "-e", "https://b23.tv"},
			method: http.MethodGet,
			url:    "https://httpbin.org/get",
			header: map[string]string{"User-Agent": "Nana7mi", "Referer": "https://b23.tv"},
		},
		{
			name:   "-b 带等号才当 Cookie",
			args:   []string{"https://httpbin.org/get", "-b", "a=1", "-b", "b=2"},
			method: http.MethodGet,
			url:    "https://httpbin.org/get",
			cookie: "a=1;b=2",
		},
		{
			// 不带等号的 -b 是 curl 的 cookie 文件写法，这里忽略
			name:   "-b 不带等号被忽略",
			args:   []string{"https://httpbin.org/get", "-b", "cookies.txt"},
			method: http.MethodGet,
			url:    "https://httpbin.org/get",
			cookie: "",
		},
		{
			name:    "没有 URL 报错",
			args:    []string{"-X", "GET"},
			wantErr: true,
		},
		{
			name:    "空参数报错",
			args:    nil,
			wantErr: true,
		},
		{
			name:    "无法识别的参数报错",
			args:    []string{"whatever"},
			wantErr: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req, err := argsToRequest(ctx, c.args...)
			if c.wantErr {
				if err == nil {
					t.Fatalf("应当报错，得到 %+v", req)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if req.Method != c.method {
				t.Errorf("Method = %q，期望 %q", req.Method, c.method)
			}
			if got := req.URL.String(); got != c.url {
				t.Errorf("URL = %q，期望 %q", got, c.url)
			}
			if got := readBody(t, req); got != c.body {
				t.Errorf("body = %q，期望 %q", got, c.body)
			}
			for k, v := range c.header {
				if got := req.Header.Get(k); got != v {
					t.Errorf("Header %q = %q，期望 %q", k, got, v)
				}
			}
			if got := req.Header.Get("Cookie"); got != c.cookie {
				t.Errorf("Cookie = %q，期望 %q", got, c.cookie)
			}
		})
	}
}

// TestContentType 表单体自动补 Content-Type，显式设置优先
func TestContentType(t *testing.T) {
	req, err := argsToRequest(context.Background(), "https://httpbin.org/post", "-d", "a=1")
	if err != nil {
		t.Fatal(err)
	}
	if got := req.Header.Get("Content-Type"); got != "application/x-www-form-urlencoded" {
		t.Errorf("自动 Content-Type = %q", got)
	}

	req, err = argsToRequest(context.Background(),
		"https://httpbin.org/post", "-d", "{\"a\":1}", "-H", "Content-Type: application/json")
	if err != nil {
		t.Fatal(err)
	}
	if got := req.Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("显式 Content-Type 应优先，得到 %q", got)
	}
}

// TestBasicAuth -u user:pass 转 Basic 认证；没有冒号则不设置
func TestBasicAuth(t *testing.T) {
	req, err := argsToRequest(context.Background(), "https://httpbin.org/get", "-u", "nana:7mi")
	if err != nil {
		t.Fatal(err)
	}
	user, pass, ok := req.BasicAuth()
	if !ok || user != "nana" || pass != "7mi" {
		t.Errorf("BasicAuth = (%q, %q, %v)", user, pass, ok)
	}

	req, err = argsToRequest(context.Background(), "https://httpbin.org/get", "-u", "nopass")
	if err != nil {
		t.Fatal(err)
	}
	if req.Header.Get("Authorization") != "" {
		t.Errorf("没有冒号不该设置 Authorization，得到 %q", req.Header.Get("Authorization"))
	}
}

// TestCURLToRequest 走 shellwords 的真实命令行解析
func TestCURLToRequest(t *testing.T) {
	ctx := context.Background()
	req, err := CURLToRequest(ctx, "'https://httpbin.org/get?a=1' -H 'X-Test: hello' -A 'Nana7mi'")
	if err != nil {
		t.Fatal(err)
	}
	if req.URL.String() != "https://httpbin.org/get?a=1" {
		t.Errorf("URL = %q", req.URL.String())
	}
	if req.Header.Get("X-Test") != "hello" || req.Header.Get("User-Agent") != "Nana7mi" {
		t.Errorf("headers = %v", req.Header)
	}

	// 允许直接粘贴完整命令（带 curl 前缀）
	req, err = CURLToRequest(ctx, "curl 'https://httpbin.org/get' -H 'X-Test: hello'")
	if err != nil {
		t.Fatalf("带 curl 前缀应当可用: %v", err)
	}
	if req.URL.String() != "https://httpbin.org/get" || req.Header.Get("X-Test") != "hello" {
		t.Errorf("带前缀解析结果不对: %v %v", req.URL, req.Header)
	}

	// 引号不配对时 shellwords 报错
	if _, err := CURLToRequest(ctx, "'https://httpbin.org/get"); err == nil {
		t.Error("引号不配对应当报错")
	}
}

// TestArgsToRequestCtx ctx 会被带到请求上
func TestArgsToRequestCtx(t *testing.T) {
	type key struct{}
	ctx := context.WithValue(context.Background(), key{}, "v")
	req, err := ArgsToRequest(ctx, "https://httpbin.org/get")
	if err != nil {
		t.Fatal(err)
	}
	if req.Context().Value(key{}) != "v" {
		t.Error("ctx 没传到请求上")
	}
}

// TestHeaderNameCase 头名大小写不敏感（Add 会规范化）
func TestHeaderNameCase(t *testing.T) {
	req, err := argsToRequest(context.Background(), "https://httpbin.org/get", "-H", "x-lower: 1")
	if err != nil {
		t.Fatal(err)
	}
	if got := req.Header.Get("X-Lower"); got != "1" {
		t.Errorf("规范化后取不到: %v", req.Header)
	}
}
