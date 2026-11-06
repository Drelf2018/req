package req

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/Drelf2018/req/method"
)

// Session 会话
type Session struct {
	http.Client

	// 基础路径，若 API 路径以 "/" 开头则会拼接在此路径后
	BaseURL *url.URL

	// 默认请求头，会自动为每个请求添加
	Header http.Header
}

// NewSession 新建会话
func NewSession(opts ...func(*Session) error) *Session {
	s := &Session{}
	for _, opt := range opts {
		err := opt(s)
		if err != nil {
			panic(err)
		}
	}
	return s
}

// Clone 克隆一个会话
func (s *Session) Clone() *Session {
	session := &Session{
		Client: s.Client,
		Header: s.Header.Clone(),
	}
	if s.BaseURL != nil {
		baseURL := *s.BaseURL
		session.BaseURL = &baseURL
	}
	return session
}

// Authorization 获取已设置的默认 Authorization 请求头
func (s *Session) Authorization() string {
	if s.Header == nil {
		return ""
	}
	return s.Header.Get("Authorization")
}

// UserAgent 获取已设置的默认 User-Agent 请求头
func (s *Session) UserAgent() string {
	if s.Header == nil {
		return ""
	}
	return s.Header.Get("User-Agent")
}

// JoinPath 拼接会话里的基础路径和 rawURL ，如果后者不是以 "/" 开头会直接返回其解析结果
func (s *Session) JoinPath(rawURL string) (*url.URL, error) {
	if s.BaseURL == nil || !strings.HasPrefix(rawURL, "/") {
		return url.Parse(rawURL)
	}
	// Parse first: path.Join should only handle the path, so query and fragment
	// are carried back over to the copy of the base URL.
	ref, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	u := *s.BaseURL
	u.RawPath = ""
	prefix := s.BaseURL.EscapedPath()
	if !strings.HasPrefix(prefix, "/") {
		// Return a relative path if u is relative,
		// but ensure that it contains no ../ elements.
		u.Path = path.Join("/"+prefix, ref.Path)[1:]
	} else {
		u.Path = path.Join(prefix, ref.Path)
	}
	// path.Join will remove any trailing slashes.
	// Preserve at least one.
	if strings.HasSuffix(ref.Path, "/") && !strings.HasSuffix(u.Path, "/") {
		u.Path += "/"
	}
	// Keep the base's own query if the reference does not have one.
	if ref.RawQuery != "" {
		u.RawQuery = ref.RawQuery
	}
	if ref.Fragment != "" {
		u.Fragment, u.RawFragment = ref.Fragment, ""
	}
	return &u, nil
}

// NewRequestWithContext 携带上下文新建请求
func (s *Session) NewRequestWithContext(ctx context.Context, api API) (r *http.Request, err error) {
	// 提取 API 中字段
	task, err := method.LoadTask(api)
	if err != nil {
		return
	}
	// 设置默认请求头
	reader := &http.Request{}
	if s.Header != nil {
		reader.Header = s.Header.Clone()
	} else {
		reader.Header = make(http.Header)
	}
	// 设置 Cookie
	u, err := s.JoinPath(api.RawURL())
	if err != nil {
		return
	}
	if s.Jar != nil {
		for _, cookie := range s.Jar.Cookies(u) {
			reader.AddCookie(cookie)
		}
	}
	if jar := ToInterface[http.CookieJar](api); jar != nil {
		for _, cookie := range jar.Cookies(u) {
			reader.AddCookie(cookie)
		}
	}
	var cookies []*http.Cookie
	if cookie, ok := api.(method.APICookie); ok {
		cookies, err = cookie.Cookie(ctx, api, reader, task.Cookie)
	} else {
		cookies, err = method.MakeCookies(ctx, api, reader, task.Cookie)
	}
	if err != nil {
		return
	}
	for _, c := range cookies {
		reader.AddCookie(c)
	}
	// 覆盖请求头
	var userHeader http.Header
	if header, ok := api.(method.APIHeader); ok {
		userHeader, err = header.Header(ctx, api, reader, task.Header)
	} else {
		userHeader, err = method.MakeHeader(ctx, api, reader, task.Header)
	}
	if err != nil {
		return
	}
	for k, vs := range userHeader {
		reader.Header[http.CanonicalHeaderKey(k)] = vs
	}
	// 获取请求体
	var b io.Reader
	var bodyHeader http.Header
	if body, ok := api.(method.APIBody); ok {
		b, bodyHeader, err = body.Body(ctx, api, reader, task.Body)
	} else if len(task.Body) > 0 {
		b, bodyHeader, err = method.PostJSON{}.Body(ctx, api, reader, task.Body)
	}
	if err != nil {
		return
	}
	// 更新请求头
	for k, vs := range bodyHeader {
		reader.Header[http.CanonicalHeaderKey(k)] = vs
	}
	// 新建请求
	r, err = http.NewRequestWithContext(ctx, api.Method(), u.String(), b)
	if err != nil {
		return
	}
	r.Header = reader.Header
	// 设置请求参数
	var q string
	if query, ok := api.(method.APIQuery); ok {
		q, err = query.Query(ctx, api, reader, task.Query)
	} else {
		q, err = method.MakeQuery(ctx, api, reader, task.Query)
	}
	if err != nil {
		return
	}
	if q != "" {
		if r.URL.RawQuery != "" {
			r.URL.RawQuery += "&" + q
		} else {
			r.URL.RawQuery = q
		}
	}
	// 设置自定义参数
	if custom, ok := api.(method.APICustom); ok {
		err = custom.Custom(ctx, api, r, task.Custom)
	}
	return
}

// NewRequest 新建请求
func (s *Session) NewRequest(api API) (r *http.Request, err error) {
	return s.NewRequestWithContext(context.Background(), api)
}

// SetOnlyCookieJar 是一个不返回 Cookies 的 http.CookieJar ，仅可用于 SetCookies
type SetOnlyCookieJar struct {
	http.CookieJar
}

// Cookies 永远返回空值
func (SetOnlyCookieJar) Cookies(u *url.URL) []*http.Cookie { return nil }

// BeforeRequest 请求前钩子
type BeforeRequest interface {
	BeforeRequest(*http.Client, *http.Request, API) error
}

// AfterResponse 响应后钩子，用于检验状态码，出现错误时必须自行关闭响应体
type AfterResponse interface {
	AfterResponse(*http.Client, *http.Response, API) error
}

// do 发送一次请求
func (s *Session) do(ctx context.Context, api API) (resp *http.Response, err error) {
	// 新建请求
	r, err := s.NewRequestWithContext(ctx, api)
	if err != nil {
		return
	}
	// 创建 Client 拷贝，用于独立设置 Jar、Timeout 等
	clientCopy := s.Client
	cli := &clientCopy
	// 设置 CookieJar
	if jar := ToInterface[http.CookieJar](api); jar != nil {
		cli.Jar = SetOnlyCookieJar{CookieJar: jar}
	} else if s.Jar != nil {
		cli.Jar = SetOnlyCookieJar{CookieJar: s.Jar}
	}
	// 发送请求
	if before, ok := api.(BeforeRequest); ok {
		err = before.BeforeRequest(cli, r, api)
		if err != nil {
			return
		}
	}
	resp, err = cli.Do(r)
	if err != nil {
		return
	}
	// 检验响应
	if after, ok := api.(AfterResponse); ok {
		err = after.AfterResponse(cli, resp, api)
	} else if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// 错误响应可能是一整页 HTML，因此只读前 4KB
		const limit = 4 << 10
		body, _ := io.ReadAll(io.LimitReader(resp.Body, limit+1))
		resp.Body.Close()
		msg := string(body)
		if len(body) > limit {
			msg = msg[:limit] + "..."
		}
		err = fmt.Errorf("req: failed to request: %s (%s)", msg, resp.Status)
	}
	return
}

// DoWithContext 携带上下文发送请求
func (s *Session) DoWithContext(ctx context.Context, api API) (*http.Response, error) {
	resp, err := s.do(ctx, api)
	if err == nil {
		return resp, nil
	}
	delayer := ToInterface[Delayer](api)
	if delayer == nil {
		return nil, err
	}
	ticker := NewDelayTicker(delayer)
	defer ticker.Stop()
	for {
		select {
		case _, ok := <-ticker.C:
			// 延迟器到期时 ticker 会关掉通道
			if !ok {
				return nil, err
			}
			// 否则继续重试
			resp, err = s.do(ctx, api)
			if err == nil {
				return resp, nil
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// Do 发送请求
func (s *Session) Do(api API) (*http.Response, error) {
	return s.DoWithContext(context.Background(), api)
}

// ContentWithContext 携带上下文获取请求结果
func (s *Session) ContentWithContext(ctx context.Context, api API) ([]byte, error) {
	resp, err := s.DoWithContext(ctx, api)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// Content 获取请求结果
func (s *Session) Content(api API) ([]byte, error) {
	return s.ContentWithContext(context.Background(), api)
}

// TextWithContext 携带上下文获取请求结果字符串
func (s *Session) TextWithContext(ctx context.Context, api API) (string, error) {
	b, err := s.ContentWithContext(ctx, api)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Text 获取请求结果字符串
func (s *Session) Text(api API) (string, error) {
	return s.TextWithContext(context.Background(), api)
}

// WriteWithContext 携带上下文将请求结果写入文件
func (s *Session) WriteWithContext(ctx context.Context, api API, name string, perm os.FileMode) error {
	resp, err := s.DoWithContext(ctx, api)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	// 写入文件
	f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, resp.Body)
	if err1 := f.Close(); err1 != nil && err == nil {
		err = err1
	}
	return err
}

// Write 将请求结果写入文件
func (s *Session) Write(api API, name string, perm os.FileMode) error {
	return s.WriteWithContext(context.Background(), api, name, perm)
}

// Unwrap 用于从响应体中解包出错误
type Unwrap interface {
	Unwrap() error
}

// ResultWithContext 携带上下文将请求结果以 JSON 格式反序列化进对象，该对象必须是指针
func (s *Session) ResultWithContext(ctx context.Context, api API, result any) (err error) {
	resp, err := s.DoWithContext(ctx, api)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	// 反序列化
	err = json.NewDecoder(resp.Body).Decode(result)
	if err != nil {
		return
	}
	// 解包错误
	if i, ok := result.(Unwrap); ok {
		err = i.Unwrap()
	}
	return
}

// Result 将请求结果以 JSON 格式反序列化进对象，该对象必须是指针
func (s *Session) Result(api API, result any) error {
	return s.ResultWithContext(context.Background(), api, result)
}

// JSONWithContext 携带上下文将请求结果以 JSON 格式反序列化进接口
func (s *Session) JSONWithContext(ctx context.Context, api API) (data any, err error) {
	err = s.ResultWithContext(ctx, api, &data)
	return
}

// JSON 将请求结果以 JSON 格式反序列化进接口
func (s *Session) JSON(api API) (any, error) {
	return s.JSONWithContext(context.Background(), api)
}
