package req

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strings"
)

// 客户端
type Client struct {
	http.Client
	BaseURL   *url.URL
	Header    http.Header
	Variables map[string]any // Client will use the value in Variables when the Field's Value starts with "$"
}

// 获取 Variables 中的值
//
// 参数 key 必须以 "$" 开头
func (c *Client) Value(key string) any {
	if c.Variables == nil {
		return nil
	}
	if strings.HasPrefix(key, "$") {
		return c.Variables[key]
	}
	return nil
}

// 获取 Variables 中的值并转换成字符串
func (c *Client) ValueString(key string) (string, error) {
	i := c.Value(key)
	if i != nil {
		return Marshal(i)
	}
	return key, nil
}

// 设置默认请求头 Authorization
func (c *Client) SetAuthorization(auth string) {
	if c.Header == nil {
		c.Header = make(http.Header)
	}
	c.Header.Set("Authorization", auth)
}

// 获取默认请求头 Authorization
func (c *Client) Authorization() (auth string) {
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

// 拼接 BaseURL 和提供的 rawURL
//
// 当 rawURL 以 "/" 开头时才会拼接
func (c *Client) URL(rawURL string) string {
	if c.BaseURL != nil && strings.HasPrefix(rawURL, "/") {
		rawURL = c.BaseURL.JoinPath(rawURL).String()
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
			s, err := c.ValueString(data.Value)
			if err != nil {
				return err
			}
			adder.Add(data.Name, s)
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

// 获取 *http.Request 对象
func (c *Client) AddBody(ctx context.Context, api APIData, body io.Reader) (*http.Request, error) {
	return http.NewRequestWithContext(ctx, api.Method(), c.URL(api.RawURL()), body)
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

func do(c http.Client, req *http.Request, jar CookieJar) (*http.Response, error) {
	c.Jar = jar
	return c.Do(req)
}

// 发送带上下文的请求
func (c *Client) DoWithContext(ctx context.Context, api API) (*http.Response, error) {
	req, err := api.NewRequestWithContext(ctx, c, api)
	if err != nil {
		return nil, err
	}
	if jar, ok := api.(CookieJar); ok && jar.IsValid() {
		return do(c.Client, req, jar)
	}
	return c.Client.Do(req)
}

// 发送请求
func (c *Client) Do(api API) (*http.Response, error) {
	return c.DoWithContext(context.Background(), api)
}

// 获取带上下文的请求结果
func (c *Client) ContentWithContext(ctx context.Context, api API) ([]byte, error) {
	resp, err := c.DoWithContext(ctx, api)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// 获取请求结果
func (c *Client) Content(api API) ([]byte, error) {
	resp, err := c.Do(api)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
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

// 生成 cURL
func (c *Client) CURL(api API) (string, error) {
	req, err := api.NewRequestWithContext(context.Background(), c, api)
	if err != nil {
		return "", err
	}
	w := bytes.NewBufferString("curl")
	if req.Body != nil {
		defer req.Body.Close()
		w.WriteString(" -d '")
		_, err = w.ReadFrom(req.Body)
		if err != nil {
			return "", err
		}
		w.WriteByte('\'')
	}
	for key, values := range req.Header {
		for _, value := range values {
			w.WriteString(" -H '")
			w.WriteString(key)
			w.WriteString(": ")
			w.WriteString(value)
			w.WriteByte('\'')
		}
	}
	if req.Method != http.MethodGet {
		w.WriteString(" -X ")
		w.WriteString(req.Method)
	}
	w.WriteByte(' ')
	w.WriteString(req.URL.String())
	return w.String(), nil
}

// 将请求结果改写成结构体
func (c *Client) Struct(api API, name string) ([]byte, error) {
	b, err := c.Content(api)
	if err != nil {
		return nil, err
	}
	return NewConverter(true, true).JSONToStruct(b, name)
}

// 在指定文件写入请求结果改写的结构体
func (c *Client) Generate(filename string, api API) error {
	name := reflect.TypeOf(api).Name()
	b, err := c.Struct(api, name+"Response")
	if err != nil {
		return err
	}
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, os.ModePerm)
	if err != nil {
		return err
	}
	f.Write([]byte{'\n'})
	_, err = f.Write(b)
	if err != nil {
		return err
	}
	f.Write([]byte{'\n', '\n'})
	m := api.Method()
	f.WriteString(fmt.Sprintf(`func %s%s() (result %sResponse, err error) {
	err = cli.Result(%s{}, &result)
	return
}`, strings.ToUpper(m[:1])+strings.ToLower(m[1:]), name, name, name))
	return f.Close()
}

// 克隆客户端
func (c *Client) Clone(rawURL string) (*Client, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	return &Client{
		Client:    c.Client,
		BaseURL:   u,
		Header:    c.Header.Clone(),
		Variables: c.Variables,
	}, nil
}

// 必须克隆！
func (c *Client) MustClone(rawURL string) *Client {
	cli, err := c.Clone(rawURL)
	if err != nil {
		panic(err)
	}
	return cli
}
