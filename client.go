package req

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path"
	"reflect"
	"strings"
	"time"
)

// 客户端
type Client struct {
	http.Client

	// 基础路径
	// 若 API 路径以 "/" 开头则会拼接在此路径后
	BaseURL *url.URL

	// 默认请求头
	Header http.Header

	// 自定义变量
	// 当字段 tag 中的值以 "$" 开头则会尝试在该字典中查找对应值
	Variables map[string]any
}

// 设置 Variables 中的值
func (c *Client) Set(key string, value any) {
	if c.Variables == nil {
		c.Variables = make(map[string]any)
	}
	c.Variables[key] = value
}

// 获取 Variables 中的值
//
// 参数 key 必须以 "$" 开头
func (c *Client) Value(key string) any {
	if c.Variables == nil || !strings.HasPrefix(key, "$") {
		return nil
	}
	return c.Variables[key]
}

// 设置默认请求头 Authorization
func (c *Client) SetAuthorization(auth string) {
	if c.Header == nil {
		c.Header = make(http.Header)
	}
	c.Header.Set("Authorization", auth)
}

// 获取默认请求头 Authorization
func (c *Client) Authorization() string {
	if c.Header == nil {
		return ""
	}
	return c.Header.Get("Authorization")
}

// 设置默认请求头 User-Agent
func (c *Client) SetUserAgent(val string) {
	if c.Header == nil {
		c.Header = make(http.Header)
	}
	c.Header.Set("User-Agent", val)
}

// 获取默认请求头 User-Agent
func (c *Client) UserAgent() string {
	if c.Header == nil {
		return ""
	}
	return c.Header.Get("User-Agent")
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

// 拼接 BaseURL 和提供的 rawURL
//
// 当 rawURL 以 "/" 开头时才会拼接
func (c *Client) URL(rawURL string) string {
	if c.BaseURL != nil && strings.HasPrefix(rawURL, "/") {
		return JoinPath(c.BaseURL, rawURL).String()
	}
	return rawURL
}

// 向 Adder 接口添加数据
func (c *Client) AddValue(adder Adder, data Field, v reflect.Value) error {
	field, err := v.FieldByIndexErr(data.Index)
	if err != nil {
		return err
	}

	if field.IsZero() {
		if data.Omitempty {
			return nil
		}
		if data.Value != "" {
			i := c.Value(data.Value)
			if i != nil {
				s, err := Marshal(i)
				if err != nil {
					return err
				}
				adder.Add(data.Name, s)
			} else {
				adder.Add(data.Name, data.Value)
			}
			return nil
		}
	}

	switch field.Kind() {
	case reflect.Array, reflect.Slice:
		for i := 0; i < field.Len(); i++ {
			s, err := Marshal(field.Index(i).Interface())
			if err != nil {
				return err
			}
			adder.Add(data.Name, s)
		}
	default:
		s, err := Marshal(field.Interface())
		if err != nil {
			return err
		}
		adder.Add(data.Name, s)
	}

	return nil
}

// 向 req 请求添加请求参数
func (c *Client) AddQuery(req *http.Request, query []Field, val reflect.Value) (err error) {
	q := make(url.Values)
	for _, data := range query {
		err = c.AddValue(q, data, val)
		if err != nil {
			return
		}
	}
	if len(q) != 0 {
		req.URL.RawQuery = q.Encode()
	}
	return
}

// 向 req 请求添加请求头
//
// 会使用 Client 中设置的默认请求头
func (c *Client) AddHeader(req *http.Request, header []Field, val reflect.Value) (err error) {
	if c.Header != nil {
		req.Header = c.Header.Clone()
	}
	for _, data := range header {
		req.Header[data.Name] = []string{}
		err = c.AddValue(req.Header, data, val)
		if err != nil {
			return
		}
	}
	return
}

// 根据提供的 []Field 制作 url.Values
func (c *Client) MakeURLValues(fields []Field, val reflect.Value) (v url.Values, err error) {
	v = make(url.Values)
	for _, data := range fields {
		err = c.AddValue(v, data, val)
		if err != nil {
			return
		}
	}
	return
}

// 根据提供的 []Field 制作 map[string]any
func (c *Client) MakeJSONMap(body []Field, value reflect.Value) (m map[string]any, err error) {
	m = make(map[string]any)
	var field reflect.Value
	for _, data := range body {
		field, err = value.FieldByIndexErr(data.Index)
		if err != nil {
			return
		}
		if field.IsZero() {
			if data.Omitempty {
				continue
			}
			if data.Value != "" {
				i := c.Value(data.Value)
				if i != nil {
					m[data.Name] = i
				} else {
					m[data.Name] = data.Value
				}
				continue
			}
		}
		m[data.Name] = field.Interface()
	}
	return
}

func (c *Client) newRequest(ctx context.Context, api API, task *Task, value reflect.Value) (req *http.Request, err error) {
	// 获取请求体
	var r io.Reader
	if body, ok := api.(APIBody); ok {
		r, err = body.Body(c, task.Body, value, api)
	} else if api.Method() == http.MethodPost {
		r, err = PostJSON{}.Body(c, task.Body, value, api)
	}
	if err != nil {
		return
	}
	// 新建请求
	req, err = http.NewRequestWithContext(ctx, api.Method(), c.URL(api.RawURL()), r)
	if err != nil {
		return
	}
	// 获取请求参数
	if query, ok := api.(APIQuery); ok {
		err = query.Query(req, c, task.Query, value, api)
	} else {
		err = c.AddQuery(req, task.Query, value)
	}
	if err != nil {
		return
	}
	// 获取请求头
	if header, ok := api.(APIHeader); ok {
		err = header.Header(req, c, task.Header, value, api)
	} else {
		err = c.AddHeader(req, task.Header, value)
	}
	return
}

// 新建带上下文的请求
func (c *Client) NewRequestWithContext(ctx context.Context, api API) (req *http.Request, err error) {
	return c.newRequest(ctx, api, LoadTask(api), reflect.Indirect(reflect.ValueOf(api)))
}

// 新建请求
func (c *Client) NewRequest(api API) (req *http.Request, err error) {
	return c.NewRequestWithContext(context.Background(), api)
}

// 发送带上下文的请求
func (c *Client) DoWithContext(ctx context.Context, api API) (resp *http.Response, err error) {
	// 提取 API 中字段
	task := LoadTask(api)
	// 获取 API 的值(reflect.Value)以便后续添加参数
	value := reflect.Indirect(reflect.ValueOf(api))
	// 新建请求
	req, err := c.newRequest(ctx, api, task, value)
	if err != nil {
		return
	}
	// 初始化 CookieJar
	cli := c.Client
	if cookieJar, ok := api.(http.CookieJar); ok {
		cli.Jar = cookieJar
	}
	// 添加字段中 cookie
	if len(task.Cookie) != 0 {
		if cli.Jar == nil {
			cli.Jar, _ = cookiejar.New(nil)
		}
		adder := FuncAdder(func(key, val string) { cli.Jar.SetCookies(req.URL, []*http.Cookie{{Name: key, Value: val}}) })
		for _, data := range task.Cookie {
			err = c.AddValue(adder, data, value)
			if err != nil {
				return
			}
		}
	}
	// 发送请求
	hook, isHook := api.(BeforeRequest)
	if isHook {
		hook.BeforeRequest(req, c, api, 0)
	}
	resp, err = cli.Do(req)
	// 检验响应
	checker, isChecker := api.(CheckResponse)
	if err == nil {
		if isChecker {
			err = checker.CheckResponse(resp, c, api, 0)
		} else if resp.StatusCode != 200 {
			err = fmt.Errorf("http: response status: %s", resp.Status)
		}
	}
	// 重试
	if err != nil {
		if ticker, ok := api.(RetryTicker); ok {
			for i := 0; err != nil; i++ {
				delay, ok := ticker.NextRetry(i)
				if !ok {
					break
				}
				time.Sleep(delay)
				if isHook {
					hook.BeforeRequest(req, c, api, i+1)
				}
				resp, err = cli.Do(req)
				if err == nil {
					if isChecker {
						err = checker.CheckResponse(resp, c, api, i+1)
					} else if resp.StatusCode != 200 {
						err = fmt.Errorf("http: response status: %s", resp.Status)
					}
				}
			}
		}
	}
	return
}

// 发送请求
func (c *Client) Do(api API) (*http.Response, error) {
	return c.DoWithContext(context.Background(), api)
}

// 获取带上下文的请求结果
func (c *Client) ContentWithContext(ctx context.Context, api API) (p []byte, err error) {
	resp, err := c.DoWithContext(ctx, api)
	if err != nil {
		return nil, err
	}
	p, err = io.ReadAll(resp.Body)
	resp.Body.Close()
	return
}

// 获取请求结果
func (c *Client) Content(api API) (p []byte, err error) {
	resp, err := c.Do(api)
	if err != nil {
		return nil, err
	}
	p, err = io.ReadAll(resp.Body)
	resp.Body.Close()
	return
}

// 获取带上下文的请求结果字符串
func (c *Client) TextWithContext(ctx context.Context, api API) (string, error) {
	p, err := c.ContentWithContext(ctx, api)
	if err != nil {
		return "", err
	}
	return string(p), nil
}

// 获取请求结果字符串
func (c *Client) Text(api API) (string, error) {
	p, err := c.Content(api)
	if err != nil {
		return "", err
	}
	return string(p), nil
}

// 将带上下文的请求结果写入文件
func (c *Client) WriteWithContext(ctx context.Context, api API, name string, perm os.FileMode) error {
	p, err := c.ContentWithContext(ctx, api)
	if err != nil {
		return err
	}
	return os.WriteFile(name, p, perm)
}

// 将请求结果写入文件
func (c *Client) Write(api API, name string, perm os.FileMode) error {
	p, err := c.Content(api)
	if err != nil {
		return err
	}
	return os.WriteFile(name, p, perm)
}

// 将带上下文的请求结果以 JSON 格式解析进对象
//
// result 必须是指针
func (c *Client) ResultWithContext(ctx context.Context, api API, result any) (err error) {
	resp, err := c.DoWithContext(ctx, api)
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

// 将请求结果以 JSON 格式解析进对象
//
// result 必须是指针
func (c *Client) Result(api API, result any) error {
	return c.ResultWithContext(context.Background(), api, result)
}

// 将带上下文的请求结果以 JSON 格式解析进接口
func (c *Client) JSONWithContext(ctx context.Context, api API) (data any, err error) {
	err = c.ResultWithContext(ctx, api, &data)
	return
}

// 将请求结果以 JSON 格式解析进接口
func (c *Client) JSON(api API) (data any, err error) {
	err = c.Result(api, &data)
	return
}

type ClientOption interface {
	ClientOption(cli *Client) error
}

type ClientURL string

func (c ClientURL) ClientOption(cli *Client) (err error) {
	cli.BaseURL, err = url.Parse(string(c))
	return
}

var _ ClientOption = ClientURL("")

type ClientHeaders map[string]string

func (c ClientHeaders) ClientOption(cli *Client) error {
	if cli.Header == nil {
		cli.Header = make(http.Header)
	}
	for k, v := range c {
		cli.Header.Set(k, v)
	}
	return nil
}

var _ ClientOption = ClientHeaders{}

func ClientHeader(key, val string) ClientHeaders {
	return ClientHeaders{key: val}
}

type ClientVariables map[string]string

func (c ClientVariables) ClientOption(cli *Client) error {
	if cli.Variables == nil {
		cli.Variables = make(map[string]any)
	}
	for k, v := range c {
		cli.Variables[k] = v
	}
	return nil
}

var _ ClientOption = ClientVariables{}

func NewClient(opts ...ClientOption) (*Client, error) {
	c := &Client{}
	for _, opt := range opts {
		err := opt.ClientOption(c)
		if err != nil {
			return nil, err
		}
	}
	return c, nil
}
