package req

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"reflect"
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

	// 自定义变量，当字段标签 default 中的值以 "$" 开头则会尝试在该字典中查找对应值
	Variables map[string]any
}

// MustParseURL 强制解析路径
func MustParseURL(rawURL string) *url.URL {
	u, err := url.Parse(rawURL)
	if err != nil {
		panic(err)
	}
	return u
}

// ErrInvalidKeyPrefix 无效的自定义变量名
var ErrInvalidKeyPrefix = errors.New("req: key must start with '$'")

// Set 设置自定义变量的值
func (s *Session) Set(key string, value any) error {
	if s.Variables == nil {
		s.Variables = make(map[string]any)
	}
	if !strings.HasPrefix(key, "$") {
		return ErrInvalidKeyPrefix
	}
	s.Variables[key] = value
	return nil
}

// Value 获取自定义变量的值，变量名必须以 "$" 开头
func (s *Session) Value(key string) any {
	if s.Variables != nil && strings.HasPrefix(key, "$") {
		return s.Variables[key]
	}
	return nil
}

// SetAuthorization 设置默认 Authorization 请求头
func (s *Session) SetAuthorization(auth string) {
	if s.Header == nil {
		s.Header = make(http.Header)
	}
	s.Header.Set("Authorization", auth)
}

// Authorization 获取已设置的默认 Authorization 请求头
func (s *Session) Authorization() string {
	if s.Header == nil {
		return ""
	}
	return s.Header.Get("Authorization")
}

// SetUserAgent 设置默认 User-Agent 请求头
func (s *Session) SetUserAgent(val string) {
	if s.Header == nil {
		s.Header = make(http.Header)
	}
	s.Header.Set("User-Agent", val)
}

// UserAgent 获取已设置的默认 User-Agent 请求头
func (s *Session) UserAgent() string {
	if s.Header == nil {
		return ""
	}
	return s.Header.Get("User-Agent")
}

// URL 拼接基础路径和 rawURL ，如果后者不是以 "/" 开头会直接返回其原值
func (s *Session) URL(rawURL string) string {
	if s.BaseURL == nil || !strings.HasPrefix(rawURL, "/") {
		return rawURL
	}
	var p string
	prefix := s.BaseURL.EscapedPath()
	if !strings.HasPrefix(prefix, "/") {
		// Return a relative path if u is relative,
		// but ensure that it contains no ../ elements.
		p = path.Join("/"+prefix, rawURL)[1:]
	} else {
		p = path.Join(prefix, rawURL)
	}
	// path.Join will remove any trailing slashes.
	// Preserve at least one.
	if strings.HasSuffix(rawURL, "/") && !strings.HasSuffix(p, "/") {
		p += "/"
	}
	url := *s.BaseURL
	url.Path = p
	return url.String()
}

// GetCookieJar 从对象中获取可用的 http.CookieJar ，如果仅在结构体中嵌入了字段，但值不可用，仍返回空
func GetCookieJar(v any, u *url.URL) http.CookieJar {
	if jar, ok := v.(http.CookieJar); ok {
		defer func() { recover() }()
		jar.Cookies(u)
		return jar
	}
	return nil
}

// CreateRequest 创建新请求
func (s *Session) CreateRequest(ctx context.Context, api API, task method.Task, value reflect.Value) (req *http.Request, err error) {
	// 新建请求
	req, err = http.NewRequestWithContext(ctx, api.Method(), s.URL(api.RawURL()), nil)
	if err != nil {
		return
	}
	// 设置默认请求头
	if s.Header != nil {
		req.Header = s.Header.Clone()
	}
	// 获取 Cookie
	if s.Jar != nil {
		for _, cookie := range s.Jar.Cookies(req.URL) {
			req.AddCookie(cookie)
		}
	}
	if jar := GetCookieJar(api, req.URL); jar != nil {
		for _, cookie := range jar.Cookies(req.URL) {
			req.AddCookie(cookie)
		}
	}
	if cookie, ok := api.(method.APICookie); ok {
		err = cookie.Cookie(req, value, task.Cookie)
		if err != nil {
			return
		}
	} else {
		method.AddCookie(req, value, task.Cookie)
	}
	// 设置 XSRF 请求头
	if xsrf, ok := api.(method.APIXSRF); ok {
		xsrfCookieName, xsrfHeaderName := xsrf.XSRF()
		cookie, err := req.Cookie(xsrfCookieName)
		if err == nil {
			req.Header.Set(xsrfHeaderName, cookie.Value)
		}
	}
	// 获取请求体
	var r io.Reader
	if body, ok := api.(method.APIBody); ok {
		r, err = body.Body(req, value, task.Body)
	} else if api.Method() == http.MethodPost {
		r, err = method.PostJSON{}.Body(req, value, task.Body)
		req.Header.Set("Content-Type", "application/json")
	}
	if err != nil {
		return
	}
	if rc, ok := r.(io.ReadCloser); ok {
		req.Body = rc
	} else if r != nil {
		req.Body = io.NopCloser(r)
	}
	if v, ok := r.(interface{ Len() int }); ok {
		req.ContentLength = int64(v.Len())
	}
	// 设置请求参数
	if query, ok := api.(method.APIQuery); ok {
		err = query.Query(req, value, task.Query)
		if err != nil {
			return
		}
	} else {
		method.AddQuery(req, value, task.Query)
	}
	// 设置请求头
	if header, ok := api.(method.APIHeader); ok {
		err = header.Header(req, value, task.Header)
		if err != nil {
			return
		}
	} else {
		method.AddHeader(req, value, task.Header)
	}
	// 设置自定义参数
	if custom, ok := api.(method.APICustom); ok {
		err = custom.Custom(req, value, task.Custom)
	}
	return
}

// NewRequestWithContext 携带上下文新建请求
func (s *Session) NewRequestWithContext(ctx context.Context, api API) (req *http.Request, err error) {
	return s.CreateRequest(WithMap(ctx, s.Variables), api, method.LoadTask(api), reflect.Indirect(reflect.ValueOf(api)))
}

// NewRequest 新建请求
func (s *Session) NewRequest(api API) (req *http.Request, err error) {
	return s.NewRequestWithContext(context.Background(), api)
}

// SetOnlyCookieJar 是一个不返回 Cookies 的 http.CookieJar ，仅可用于 SetCookies
type SetOnlyCookieJar struct {
	http.CookieJar
}

func (SetOnlyCookieJar) Cookies(u *url.URL) []*http.Cookie { return nil }

// DoWithContext 携带上下文发送请求
func (s *Session) DoWithContext(ctx context.Context, api API) (resp *http.Response, err error) {
	ctx = WithMap(ctx, s.Variables)
	// 提取 API 中字段
	task := method.LoadTask(api)
	// 获取 API 的值用于获取参数值
	value := reflect.Indirect(reflect.ValueOf(api))
	// 新建请求
	req, err := s.CreateRequest(ctx, api, task, value)
	if err != nil {
		return
	}
	// 创建 Client 拷贝
	clientCopy := s.Client
	cli := &clientCopy
	// 设置 CookieJar
	if jar := GetCookieJar(api, req.URL); jar != nil {
		cli.Jar = SetOnlyCookieJar{CookieJar: jar}
	} else if s.Jar != nil {
		cli.Jar = SetOnlyCookieJar{CookieJar: s.Jar}
	}
	// 发送请求
	if before, ok := api.(BeforeRequest); ok {
		err = before.BeforeRequest(cli, req, api)
		if err != nil {
			return
		}
	}
	resp, err = cli.Do(req)
	if err != nil {
		return
	}
	// 检验响应
	if checker, ok := api.(CheckResponse); ok {
		err = checker.CheckResponse(cli, resp, api)
	} else if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		err = fmt.Errorf("req: failed to request: %s (%s)", body, resp.Status)
	}
	return
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
	resp, err := s.Do(api)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// TextWithContext 携带上下文获取请求结果字符串
func (s *Session) TextWithContext(ctx context.Context, api API) (string, error) {
	p, err := s.ContentWithContext(ctx, api)
	if err != nil {
		return "", err
	}
	return string(p), nil
}

// Text 获取请求结果字符串
func (s *Session) Text(api API) (string, error) {
	p, err := s.Content(api)
	if err != nil {
		return "", err
	}
	return string(p), nil
}

// WriteWithContext 携带上下文将请求结果写入文件
func (s *Session) WriteWithContext(ctx context.Context, api API, name string, perm os.FileMode) error {
	p, err := s.ContentWithContext(ctx, api)
	if err != nil {
		return err
	}
	return os.WriteFile(name, p, perm)
}

// Write 将请求结果写入文件
func (s *Session) Write(api API, name string, perm os.FileMode) error {
	p, err := s.Content(api)
	if err != nil {
		return err
	}
	return os.WriteFile(name, p, perm)
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
func (s *Session) Result(api API, result any) (err error) {
	resp, err := s.Do(api)
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

// JSONWithContext 携带上下文将请求结果以 JSON 格式反序列化进接口
func (s *Session) JSONWithContext(ctx context.Context, api API) (data any, err error) {
	err = s.ResultWithContext(ctx, api, &data)
	return
}

// JSON 将请求结果以 JSON 格式反序列化进接口
func (s *Session) JSON(api API) (data any, err error) {
	err = s.Result(api, &data)
	return
}
