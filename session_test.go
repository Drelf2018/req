package req

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Drelf2018/req/method"
)

// newSession 起一个本地服务，把收到的请求交给 h；返回指向它的会话。
func newSession(t *testing.T, h http.HandlerFunc) *Session {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	s := NewSession(ParseURL(srv.URL + "/"))
	return s
}

// withCookies 给会话挂一个 jar，按 kv 两两喂 cookie。
func withCookies(t *testing.T, s *Session, kv ...string) *Session {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(s.BaseURL.String())
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i+1 < len(kv); i += 2 {
		jar.SetCookies(u, []*http.Cookie{{Name: kv[i], Value: kv[i+1]}})
	}
	s.Jar = jar
	return s
}

// echoJSON 一张表覆盖四个去向：body / query / header / cookie。
type echoJSON struct {
	PostJSON
	Name string `req:"body"`
	Page int    `req:"body" default:"7"`
	Tag  string `req:"query"`
	Auth string `req:"header:x-token" default:"tok"`
	Kv   string `req:"cookie:kv" default:"ck"`
}

func (*echoJSON) RawURL() string { return "/echo" }

// TestDoJSONBodyAndTargets 一条链路验四件事：
// JSON body（含 default 转成字段类型）、query、header、cookie 全部到位。
func TestDoJSONBodyAndTargets(t *testing.T) {
	var (
		mu      sync.Mutex
		seen    *http.Request
		rawBody string
	)
	s := newSession(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		seen, rawBody = r, string(b)
		mu.Unlock()
	})
	if _, err := s.Do(&echoJSON{Name: "hi", Tag: "abc"}); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if seen == nil {
		t.Fatal("服务端没收到请求")
	}
	if got := seen.Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q", got)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(rawBody), &m); err != nil {
		t.Fatalf("body 不是合法 JSON: %v：%s", err, rawBody)
	}
	if m["name"] != "hi" {
		t.Errorf("body name = %v", m["name"])
	}
	if m["page"] != float64(7) {
		t.Errorf("default 应当转成数字 7，得到 %#v (%T)", m["page"], m["page"])
	}
	if got := seen.URL.Query().Get("tag"); got != "abc" {
		t.Errorf("query tag = %q", got)
	}
	if got := seen.Header.Get("X-Token"); got != "tok" {
		t.Errorf("X-Token = %q", got)
	}
	if c, err := seen.Cookie("kv"); err != nil || c.Value != "ck" {
		t.Errorf("cookie kv = %v %v", c, err)
	}
}

// TestGetBody 建出来的请求必须自带 GetBody 和正确的 ContentLength（307/308 重放的前提）。
func TestGetBody(t *testing.T) {
	s := newSession(t, func(w http.ResponseWriter, r *http.Request) {})
	r, err := s.NewRequest(&echoJSON{Name: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	if r.GetBody == nil {
		t.Fatal("GetBody 是 nil，重定向没法重放 body")
	}
	if r.ContentLength <= 0 {
		t.Errorf("ContentLength = %d", r.ContentLength)
	}
	body, err := r.GetBody()
	if err != nil {
		t.Fatal(err)
	}
	defer body.Close()
	b, err := io.ReadAll(body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "hi") {
		t.Errorf("重放出来的 body 不对: %s", b)
	}
}

type redirectAPI struct {
	PostJSON
	Name string `req:"body"`
}

func (*redirectAPI) RawURL() string { return "/start" }

// TestRedirectKeepsBody 307 之后必须还能把 body 原样重发一遍。
func TestRedirectKeepsBody(t *testing.T) {
	var (
		mu   sync.Mutex
		got  []string
		path []string
	)
	s := newSession(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		got = append(got, string(b))
		path = append(path, r.URL.Path)
		mu.Unlock()
		if r.URL.Path == "/start" {
			http.Redirect(w, r, "/echo", http.StatusTemporaryRedirect)
		}
	})
	if _, err := s.Do(&redirectAPI{Name: "hi"}); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(got) != 2 {
		t.Fatalf("应当有两次请求（原始 + 重定向），实际 %d: %v", len(got), path)
	}
	if got[0] == "" || got[0] != got[1] {
		t.Errorf("重定向后的 body 不一致：第一次 %q，第二次 %q", got[0], got[1])
	}
}

type csrfAPI struct {
	Get
	Token string `req:"header:X-CSRF" cookie:"token"`
}

func (*csrfAPI) RawURL() string { return "/x" }

// TestCookieSourceFromJar 最典型的用法：jar 里的 cookie 抄进请求头（CSRF）。
//
// 这条在“jar 的 cookie 晚于 header 钩子挂载”的旧顺序下是坏的，
// 而 cookie 未命中若被当成致命错误，整条请求会直接失败。
func TestCookieSourceFromJar(t *testing.T) {
	var (
		mu  sync.Mutex
		got string
	)
	s := withCookies(t, newSession(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		got = r.Header.Get("X-CSRF")
		mu.Unlock()
	}), "token", "from-jar")
	if _, err := s.Do(&csrfAPI{}); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if got != "from-jar" {
		t.Errorf("X-CSRF = %q，期望从 jar 取到 from-jar", got)
	}
}

type overrideAPI struct {
	Get
}

func (*overrideAPI) RawURL() string { return "/x" }

func (*overrideAPI) Header(ctx context.Context, api API, reader method.CookieReader, header []method.Field) (http.Header, error) {
	return http.Header{"Cookie": {"override=1"}}, nil
}

// TestHeaderHookOverridesCookie header 钩子返回 Cookie 头 = 显式接管全部 cookie。
func TestHeaderHookOverridesCookie(t *testing.T) {
	var (
		mu  sync.Mutex
		got string
	)
	s := withCookies(t, newSession(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		got = r.Header.Get("Cookie")
		mu.Unlock()
	}), "token", "from-jar")
	if _, err := s.Do(&overrideAPI{}); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if strings.Contains(got, "from-jar") {
		t.Errorf("jar 的 cookie 应当被覆盖掉，得到 %q", got)
	}
	if !strings.Contains(got, "override=1") {
		t.Errorf("钩子返回的 cookie 没生效，得到 %q", got)
	}
}

type beforeAPI struct {
	Get
}

func (*beforeAPI) RawURL() string { return "/x" }

func (*beforeAPI) BeforeRequest(cli *http.Client, req *http.Request, api API) error {
	req.Header.Set("X-Before", "yes")
	return nil
}

// TestBeforeRequest BeforeRequest 能改写已经建好的请求。
func TestBeforeRequest(t *testing.T) {
	var (
		mu  sync.Mutex
		got string
	)
	s := newSession(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		got = r.Header.Get("X-Before")
		mu.Unlock()
	})
	if _, err := s.Do(&beforeAPI{}); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if got != "yes" {
		t.Errorf("X-Before = %q", got)
	}
}

// TestNonOKByDefault 没有 AfterResponse 时，非 2xx 必须报错并带上状态与响应体。
func TestNonOKByDefault(t *testing.T) {
	s := newSession(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	})
	_, err := s.Do(&beforeAPI{})
	if err == nil {
		t.Fatal("非 200 应当报错")
	}
	if !strings.Contains(err.Error(), "boom") || !strings.Contains(err.Error(), "500") {
		t.Errorf("错误信息应当带上状态码和响应体: %v", err)
	}
}

// TestAccept2xx 默认接受整个 2xx（以前只认 200，201/204 会被当成错误）
func TestAccept2xx(t *testing.T) {
	for _, code := range []int{
		http.StatusOK,
		http.StatusCreated,
		http.StatusNoContent,
		http.StatusPartialContent,
	} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			s := newSession(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(code)
				if code != http.StatusNoContent {
					_, _ = w.Write([]byte("ok"))
				}
			})
			resp, err := s.Do(&beforeAPI{})
			if err != nil {
				t.Fatalf("%d 应当算成功，得到 %v", code, err)
			}
			resp.Body.Close()
		})
	}
}

// TestWrite 响应体流式写盘（不经过 Content 的整读）
func TestWrite(t *testing.T) {
	body := strings.Repeat("x", 64<<10) // 64KB，跨多个读块
	s := newSession(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	})
	name := filepath.Join(t.TempDir(), "out.bin")
	if err := s.Write(&beforeAPI{}, name, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != body {
		t.Errorf("写入内容不对：长度 %d，期望 %d", len(got), len(body))
	}
	// 路径不可写时应当返回错误而不是 panic
	if err := s.Write(&beforeAPI{}, filepath.Join(t.TempDir(), "no", "such", "dir", "x"), 0o644); err == nil {
		t.Error("不可写路径应当报错")
	}
}

type checkAPI struct {
	Get
}

func (*checkAPI) RawURL() string { return "/x" }

func (*checkAPI) AfterResponse(cli *http.Client, resp *http.Response, api API) error {
	resp.Body.Close()
	return errors.New("看到 " + resp.Status[0:3])
}

// TestAfterResponseWins 有 AfterResponse 时由钩子说了算。
func TestAfterResponseWins(t *testing.T) {
	s := newSession(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	_, err := s.Do(&checkAPI{})
	if err == nil || err.Error() != "看到 500" {
		t.Errorf("应当由钩子决定错误，得到 %v", err)
	}
}

type formAPI struct {
	PostForm
	Name string `req:"body"`
	List []int  `req:"body"`
}

func (*formAPI) RawURL() string { return "/x" }

// TestPostForm PostForm 发的是表单编码。
func TestPostForm(t *testing.T) {
	var (
		mu      sync.Mutex
		ct, bod string
	)
	s := newSession(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		ct, bod = r.Header.Get("Content-Type"), string(b)
		mu.Unlock()
	})
	if _, err := s.Do(&formAPI{Name: "hi", List: []int{1, 2}}); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if ct != "application/x-www-form-urlencoded" {
		t.Errorf("Content-Type = %q", ct)
	}
	if bod != "list=1&list=2&name=hi" {
		t.Errorf("body = %q", bod)
	}
}

type multiAPI struct {
	PostMultipartForm
	Name string    `req:"body:name"`
	File io.Reader `req:"body:file"`
}

func (*multiAPI) RawURL() string { return "/x" }

// TestPostMultipartForm 文件字段当文件发，普通字段进表单。
func TestPostMultipartForm(t *testing.T) {
	var (
		mu   sync.Mutex
		form *multipart.Form
	)
	s := newSession(t, func(w http.ResponseWriter, r *http.Request) {
		_, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil {
			t.Errorf("Content-Type: %v", err)
			return
		}
		f, err := multipart.NewReader(r.Body, params["boundary"]).ReadForm(1 << 20)
		if err != nil {
			t.Errorf("解析 multipart: %v", err)
			return
		}
		mu.Lock()
		form = f
		mu.Unlock()
	})
	api := &multiAPI{Name: "hello", File: method.NewNamedReader("a.txt", []byte("content"))}
	if _, err := s.Do(api); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if form == nil {
		t.Fatal("服务端没解析到表单")
	}
	if got := form.Value["name"]; len(got) != 1 || got[0] != "hello" {
		t.Errorf("普通字段 = %#v", form.Value)
	}
	fh := form.File["file"]
	if len(fh) != 1 || fh[0].Filename != "a.txt" {
		t.Fatalf("文件字段 = %#v", fh)
	}
}

// TestJoinPath 钉住路径拼接的边界：query / fragment / 末尾斜杠 / 无 BaseURL / 相对路径。
// path.Join 只该看到 Path，把 "?" 或 "#' 当路径的一部分都是错的。
func TestJoinPath(t *testing.T) {
	base := NewSession(ParseURL("https://api.example.com/base/"))
	plain := &Session{Header: make(http.Header)}
	cases := []struct {
		name string
		s    *Session
		in   string
		want string
	}{
		{"普通路径", base, "/echo", "https://api.example.com/base/echo"},
		{"带 query", base, "/echo?a=1&b=2", "https://api.example.com/base/echo?a=1&b=2"},
		{"带 fragment", base, "/echo#frag", "https://api.example.com/base/echo#frag"},
		{"末尾斜杠", base, "/echo/", "https://api.example.com/base/echo/"},
		{"末尾斜杠 + query", base, "/echo/?a=1", "https://api.example.com/base/echo/?a=1"},
		{"query 里有斜杠", base, "/echo?next=/x/y", "https://api.example.com/base/echo?next=/x/y"},
		{"无 BaseURL", plain, "/echo?a=1", "/echo?a=1"},
		{"相对路径直通", base, "echo", "echo"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			u, err := c.s.JoinPath(c.in)
			if err != nil {
				t.Fatal(err)
			}
			if got := u.String(); got != c.want {
				t.Errorf("want %q，得到 %q", c.want, got)
			}
		})
	}
}

type anyMethodAPI struct {
	m    string
	Name string `req:"body"`
}

func (a *anyMethodAPI) Method() string { return a.m }
func (*anyMethodAPI) RawURL() string   { return "/echo" }

// TestBodyOnVariousMethods 没有实现 APIBody 钩子时，只要声明了 body 字段就必须发出去，
// 与 HTTP 方法无关（以前只判 POST，PUT/PATCH/DELETE 会静默丢掉整个 body）。
func TestBodyOnVariousMethods(t *testing.T) {
	for _, m := range []string{http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodGet} {
		t.Run(m, func(t *testing.T) {
			var (
				mu   sync.Mutex
				body string
				ct   string
			)
			s := newSession(t, func(w http.ResponseWriter, r *http.Request) {
				b, _ := io.ReadAll(r.Body)
				mu.Lock()
				body, ct = string(b), r.Header.Get("Content-Type")
				mu.Unlock()
			})
			if _, err := s.Do(&anyMethodAPI{m: m, Name: "hi"}); err != nil {
				t.Fatal(err)
			}
			mu.Lock()
			defer mu.Unlock()
			var got map[string]any
			if err := json.Unmarshal([]byte(body), &got); err != nil {
				t.Fatalf("body 不是合法 JSON: %v：%q", err, body)
			}
			if got["name"] != "hi" {
				t.Errorf("body name = %v", got["name"])
			}
			if ct != "application/json" {
				t.Errorf("Content-Type = %q", ct)
			}
		})
	}
}

type queryAPI struct {
	Get
	Tag string `req:"query"`
}

func (*queryAPI) RawURL() string { return "/echo?a=1&b=2" }

// TestExistingQueryPreserved RawURL 里带的 query 必须原样保留，字段参数追加在后面。
// JoinPath 若把 "?" 当成路径的一部分，这条会红。
func TestExistingQueryPreserved(t *testing.T) {
	var (
		mu  sync.Mutex
		raw string
	)
	s := newSession(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		raw = r.URL.RawQuery
		mu.Unlock()
	})
	if _, err := s.Do(&queryAPI{Tag: "x"}); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if raw != "a=1&b=2&tag=x" {
		t.Errorf("RawQuery = %q，期望 a=1&b=2&tag=x", raw)
	}
}

// seqDelayer 按预设序列给出延迟；序列用完就不再重试。
//
// 加锁是因为 DelayTicker 在自己的 goroutine 里调 NextDelay，
// 而 Stop 不会等那个 goroutine 结束，所以测试读 times 时必须持锁。
type seqDelayer struct {
	mu     sync.Mutex
	delays []time.Duration
	times  []int
}

func (d *seqDelayer) NextDelay(times int) (time.Duration, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.times = append(d.times, times)
	if times >= len(d.delays) {
		return 0, false
	}
	return d.delays[times], true
}

// snapshot 取一份 times 的副本。
func (d *seqDelayer) snapshot() []int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]int(nil), d.times...)
}

// seqAPI 通过嵌入 *seqDelayer 获得 NextDelay
type seqAPI struct {
	*seqDelayer
}

func (*seqAPI) Method() string { return http.MethodGet }
func (*seqAPI) RawURL() string { return "/x" }

// selfDelayerAPI 自己实现了 Delayer，但顺带嵌了一个没初始化的 *cookiejar.Jar
type selfDelayerAPI struct {
	*cookiejar.Jar
	seqDelayer
}

func (*selfDelayerAPI) Method() string { return http.MethodGet }
func (*selfDelayerAPI) RawURL() string { return "/x" }

// TestToInterface 只有“确实能提供该接口、且当前为 nil”的字段才算没有实现。
func TestToInterface(t *testing.T) {
	// 自己实现了 Delayer，不能被无关的 nil *cookiejar.Jar 误杀
	self := &selfDelayerAPI{}
	if ToInterface[Delayer](self) == nil {
		t.Error("自己实现了 Delayer，不该被无关的 nil 字段误杀")
	}
	if ToInterface[http.CookieJar](self) != nil {
		t.Error("nil 的 *cookiejar.Jar 应当视为没有 jar")
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	if ToInterface[http.CookieJar](&embeddedRealAPI{Jar: jar}) == nil {
		t.Error("可用的 jar 应当非 nil")
	}
	if ToInterface[http.CookieJar](&embeddedNilAPI{}) != nil {
		t.Error("nil 的 jar 应当视为没有")
	}
	if ToInterface[Delayer](&seqAPI{}) != nil {
		t.Error("nil 的 *seqDelayer 应当视为不重试")
	}
	if ToInterface[Delayer](&beforeAPI{}) != nil {
		t.Error("没实现应当返回 nil")
	}
}

// TestRetry 重试的四种行为。
func TestRetry(t *testing.T) {
	t.Run("成功即停止", func(t *testing.T) {
		var hits int32
		s := newSession(t, func(w http.ResponseWriter, r *http.Request) {
			if atomic.AddInt32(&hits, 1) < 3 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			_, _ = w.Write([]byte("ok"))
		})
		api := &seqAPI{&seqDelayer{delays: []time.Duration{0, 0}}}
		body, err := s.Content(api)
		if err != nil {
			t.Fatal(err)
		}
		if string(body) != "ok" {
			t.Errorf("body = %q", body)
		}
		if got := atomic.LoadInt32(&hits); got != 3 {
			t.Errorf("请求次数 = %d，期望 3", got)
		}
		// DelayTicker 发完一个 tick 会立刻问下一次，所以 times 可能比实际重试次数多一问
		got := api.snapshot()
		if len(got) < 2 || got[0] != 0 || got[1] != 1 {
			t.Errorf("times = %v，期望以 [0 1] 开头", got)
		}
	})

	t.Run("耗尽后返回最后一次错误", func(t *testing.T) {
		var hits int32
		s := newSession(t, func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&hits, 1)
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("boom"))
		})
		api := &seqAPI{&seqDelayer{delays: []time.Duration{0, 0}}}
		resp, err := s.Do(api)
		if err == nil || !strings.Contains(err.Error(), "boom") || !strings.Contains(err.Error(), "500") {
			t.Fatalf("应当带上最后一次的错误，得到 resp=%v err=%v", resp, err)
		}
		if got := atomic.LoadInt32(&hits); got != 3 {
			t.Errorf("请求次数 = %d，期望 3", got)
		}
		got := api.snapshot()
		if len(got) < 3 || got[0] != 0 || got[1] != 1 || got[2] != 2 {
			t.Errorf("times = %v，期望以 [0 1 2] 开头", got)
		}
	})

	t.Run("不重试", func(t *testing.T) {
		var hits int32
		s := newSession(t, func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&hits, 1)
			w.WriteHeader(http.StatusInternalServerError)
		})
		api := &seqAPI{&seqDelayer{}} // 序列为空，第一次就到期
		if _, err := s.Do(api); err == nil {
			t.Fatal("应当报错")
		}
		if got := atomic.LoadInt32(&hits); got != 1 {
			t.Errorf("请求次数 = %d，期望 1", got)
		}
		if got := api.snapshot(); len(got) != 1 || got[0] != 0 {
			t.Errorf("times = %v，期望恰好一顿 [0]", got)
		}
	})

	t.Run("等待期间被 ctx 取消", func(t *testing.T) {
		var hits int32
		s := newSession(t, func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&hits, 1)
			w.WriteHeader(http.StatusInternalServerError)
		})
		api := &seqAPI{&seqDelayer{delays: []time.Duration{time.Minute}}}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()
		_, err := s.DoWithContext(ctx, api)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("want DeadlineExceeded，得到 %v", err)
		}
		if got := atomic.LoadInt32(&hits); got != 1 {
			t.Errorf("请求次数 = %d，期望 1（首发一次后就在等待中被取消）", got)
		}
	})
}

type embeddedNilAPI struct {
	*cookiejar.Jar
}

type embeddedRealAPI struct {
	*cookiejar.Jar
}

// TestGetCookieJar 嵌入但没初始化的 jar 必须被识别成“没有 jar”，
// 而不是靠调用 Cookies 去触发 panic。
func TestGetCookieJar(t *testing.T) {
	if jar := ToInterface[http.CookieJar](&embeddedNilAPI{}); jar != nil {
		t.Errorf("嵌套 nil 的 jar 应当返回 nil，得到 %v", jar)
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := ToInterface[http.CookieJar](&embeddedRealAPI{Jar: jar}); got == nil {
		t.Error("可用的 jar 应当返回非 nil")
	}
	if got := ToInterface[http.CookieJar](&beforeAPI{}); got != nil {
		t.Errorf("没实现应当返回 nil，得到 %v", got)
	}
}
