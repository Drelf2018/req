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

// 会话
type Session struct {
	http.Client

	// 基础路径
	// 若 API 路径以 "/" 开头则会拼接在此路径后
	BaseURL *url.URL

	// 默认请求头
	Header http.Header

	// 自定义变量
	// 当字段 api tag 中的值以 "$" 开头则会尝试在该字典中查找对应值
	Variables map[string]any
}

// Set 设置 Variables 中的值
func (s *Session) Set(key string, value any) error {
	if s.Variables == nil {
		s.Variables = make(map[string]any)
	}
	if !strings.HasPrefix(key, "$") {
		return errors.New("req: key must start with '$'")
	}
	s.Variables[key] = value
	return nil
}

// Value 获取 Variables 中的值
//
// 参数 key 必须以 "$" 开头
func (s *Session) Value(key string) any {
	if s.Variables == nil {
		return nil
	}
	if strings.HasPrefix(key, "$") {
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

// JoinPath returns a new [URL] with the provided path elements joined to
// any existing path and the resulting path cleaned of any ./ or ../ elements.
// Any sequences of multiple / characters will be reduced to a single /.
func JoinPath(u *url.URL, elem ...string) *url.URL {
	elem = append([]string{u.EscapedPath()}, elem...)
	var p string
	if !strings.HasPrefix(elem[0], "/") {
		// Return a relative path if u is relative,
		// but ensure that it contains no ../ elements.
		elem[0] = "/" + elem[0]
		p = path.Join(elem...)[1:]
	} else {
		p = path.Join(elem...)
	}
	// path.Join will remove any trailing slashes.
	// Preserve at least one.
	if strings.HasSuffix(elem[len(elem)-1], "/") && !strings.HasSuffix(p, "/") {
		p += "/"
	}
	url := *u
	url.Path = p
	return &url
}

// URL 拼接 BaseURL 和提供的 rawURL
//
// 当 rawURL 以 "/" 开头时才会拼接
func (s *Session) URL(rawURL string) string {
	if s.BaseURL != nil && strings.HasPrefix(rawURL, "/") {
		return JoinPath(s.BaseURL, rawURL).String()
	}
	return rawURL
}

// CreateRequest 创建新请求
func (s *Session) CreateRequest(ctx context.Context, api API, task method.Task, value reflect.Value) (req *http.Request, err error) {
	// 获取请求体
	var r io.Reader
	if body, ok := api.(method.APIBody); ok {
		r, err = body.Body(ctx, value, task.Body)
	} else if api.Method() == http.MethodPost {
		r, err = method.PostJSON{}.Body(ctx, value, task.Body)
	}
	if err != nil {
		return
	}
	// 新建请求
	req, err = http.NewRequestWithContext(ctx, api.Method(), s.URL(api.RawURL()), r)
	if err != nil {
		return
	}
	// 获取请求参数
	if query, ok := api.(method.APIQuery); ok {
		err = query.Query(req, value, task.Query)
		if err != nil {
			return
		}
	} else {
		method.AddQuery(req, value, task.Query)
	}
	// 获取请求头
	if s.Header != nil {
		req.Header = s.Header.Clone()
	}
	if header, ok := api.(method.APIHeader); ok {
		err = header.Header(req, value, task.Header)
	} else {
		method.AddHeader(req, value, task.Header)
	}
	return
}

// NewRequestWithContext 新建带上下文的请求
func (s *Session) NewRequestWithContext(ctx context.Context, api API) (req *http.Request, err error) {
	return s.CreateRequest(WithMap(ctx, s.Variables), api, method.LoadTask(api), reflect.Indirect(reflect.ValueOf(api)))
}

// NewRequest 新建请求
func (s *Session) NewRequest(api API) (req *http.Request, err error) {
	return s.NewRequestWithContext(context.Background(), api)
}

// do 发送请求
func (s *Session) do(ctx context.Context, api API) (resp *http.Response, err error) {
	// 提取 API 中字段
	task := method.LoadTask(api)
	// 获取 API 的值用于获取参数值
	value := reflect.Indirect(reflect.ValueOf(api))
	// 新建请求
	req, err := s.CreateRequest(ctx, api, task, value)
	if err != nil {
		return nil, err
	}
	// 设置自定义参数
	if custom, ok := api.(method.APICustom); ok {
		err = custom.Custom(req, value, task.Custom)
		if err != nil {
			return
		}
	}
	// 创建 Client 拷贝
	ClientCopy := s.Client
	cli := &ClientCopy
	// 设置 CookieJar
	cli.Jar = method.NewCookieJar(req, value, task.Cookie, api)
	// 发送请求
	if before, ok := api.(BeforeRequest); ok {
		err = before.BeforeRequest(cli, req, api)
		if err != nil {
			return
		}
	}
	resp, err = cli.Do(req)
	// 检验响应
	if err == nil {
		if checker, ok := api.(CheckResponse); ok {
			err = checker.CheckResponse(cli, resp, api)
		} else if resp.StatusCode != 200 {
			err = fmt.Errorf("http: response status: %s", resp.Status)
		}
	}
	return
}

// DoWithContext 发送带上下文的请求
func (s *Session) DoWithContext(ctx context.Context, api API) (*http.Response, error) {
	return s.do(WithMap(ctx, s.Variables), api)
}

// Do 发送请求
func (s *Session) Do(api API) (*http.Response, error) {
	return s.DoWithContext(context.Background(), api)
}

// ContentWithContext 获取带上下文的请求结果
func (s *Session) ContentWithContext(ctx context.Context, api API) (p []byte, err error) {
	resp, err := s.DoWithContext(ctx, api)
	if err != nil {
		return nil, err
	}
	p, err = io.ReadAll(resp.Body)
	resp.Body.Close()
	return
}

// Content 获取请求结果
func (s *Session) Content(api API) (p []byte, err error) {
	resp, err := s.Do(api)
	if err != nil {
		return nil, err
	}
	p, err = io.ReadAll(resp.Body)
	resp.Body.Close()
	return
}

// TextWithContext 获取带上下文的请求结果字符串
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

// WriteWithContext 将带上下文的请求结果写入文件
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

// ResultWithContext 将带上下文的请求结果以 JSON 格式解析进对象
//
// result 必须是指针
func (s *Session) ResultWithContext(ctx context.Context, api API, result any) (err error) {
	resp, err := s.DoWithContext(ctx, api)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(result)
	if err != nil {
		return
	}

	if i, ok := result.(Unwrap); ok {
		err = i.Unwrap()
	}
	return
}

// Result 将请求结果以 JSON 格式解析进对象
//
// result 必须是指针
func (s *Session) Result(api API, result any) (err error) {
	resp, err := s.Do(api)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(result)
	if err != nil {
		return
	}

	if i, ok := result.(Unwrap); ok {
		err = i.Unwrap()
	}
	return
}

// JSONWithContext 将带上下文的请求结果以 JSON 格式解析进接口
func (s *Session) JSONWithContext(ctx context.Context, api API) (data any, err error) {
	err = s.ResultWithContext(ctx, api, &data)
	return
}

// JSON 将请求结果以 JSON 格式解析进接口
func (s *Session) JSON(api API) (data any, err error) {
	err = s.Result(api, &data)
	return
}

type SessionOption interface {
	SessionOption(cli *Session) error
}

type SessionURL string

func (c SessionURL) SessionOption(cli *Session) (err error) {
	cli.BaseURL, err = url.Parse(string(c))
	return
}

var _ SessionOption = SessionURL("")

type SessionHeaders map[string]string

func (c SessionHeaders) SessionOption(cli *Session) error {
	if cli.Header == nil {
		cli.Header = make(http.Header)
	}
	for k, v := range c {
		cli.Header.Set(k, v)
	}
	return nil
}

var _ SessionOption = SessionHeaders{}

func SessionHeader(key, val string) SessionHeaders {
	return SessionHeaders{key: val}
}

type SessionVariables map[string]string

func (c SessionVariables) SessionOption(cli *Session) error {
	if cli.Variables == nil {
		cli.Variables = make(map[string]any)
	}
	for k, v := range c {
		cli.Variables[k] = v
	}
	return nil
}

var _ SessionOption = SessionVariables{}

func NewSession(opts ...SessionOption) (*Session, error) {
	c := &Session{}
	for _, opt := range opts {
		err := opt.SessionOption(c)
		if err != nil {
			return nil, err
		}
	}
	return c, nil
}
