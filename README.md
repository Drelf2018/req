<p align="center">
  <a href="https://github.com/Drelf2018/webhook/">
    <img src="./avatar.jpg" height="200" alt="req" style="border-radius:50%">
  </a>
</p>

<div align="center">

# req

_✨ 通过结构体发送请求 ✨_ 

</div>

## 如何实现一个 `API`

在正式开始使用前，我们需要了解如何实现一个 `API` 接口。

```go
// API 信息
type APIData interface {
	RawURL() string
	Method() string
}

// API 构造器
type APICreator interface {
	NewRequestWithContext(ctx context.Context, cli *Client, api APIData) (*http.Request, error)
}

// API 接口
type API interface {
	APIData
	APICreator
}
```

在 [interfaces.go](./interfaces.go) 中定义了 `APIData` `APICreator` `API` 三个接口。

其中 `APIData` 是用来描述接口的，包括返回请求地址的 `RawURL() string` 方法和返回请求方式的 `Method() string` 方法，通常需要用户自行实现。

此外 `APICreator` 是用来构造底层 `*http.Request` 请求的构造器，一般不需用户自己实现。

```go
// GET 请求构造器
//
// 直接嵌入结构体即可使用
type Get struct{}

func (Get) Method() string {
	return http.MethodGet
}

// 可以通过这个方法学习如何自己实现一个构造器
func (Get) NewRequestWithContext(ctx context.Context, cli *Client, api APIData) (req *http.Request, err error) {
	// 因为是 GET 请求所以不添加 body
	req, err = cli.AddBody(ctx, api, nil)
	if err != nil {
		return
	}
	// 提取 API 中字段
	task := LoadTask(api)
	// 获取 API 的值(reflect.Value)以便后续添加参数
	value := reflect.Indirect(reflect.ValueOf(api))
	// 添加请求参数
	err = cli.AddQuery(req, task.Query, value)
	if err == nil {
		// 添加请求头
		err = cli.AddHeader(req, task.Header, value)
	}
	return
}

var _ APICreator = Get{}
```

在 [method.go](./method.go) 中定义了 `Get` 结构体，它就实现了 `Method() string` 方法和 `APICreator` 接口。如果现在难以理解可以先跳过，我们会在后面详细介绍。

## 使用

### 带有路径参数的 `GET` 请求实例

```go
type User struct {
	req.Get
	UID string
}

func (u User) RawURL() string {
	return "https://httpbin.org/anything/user/" + u.UID
}

func TestUser(t *testing.T) {
	resp, err := req.Do(User{UID: "114514"})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(string(b))
}
```

在上面代码中，我们定义了一个 `User` 结构体，并在其中嵌入了 `req.Get` 字段，代表它隐形实现了 `Method() string` 方法和 `APICreator` 接口，因此我们只需要实现 `RawURL() string` 方法。

因此我们在返回请求地址时拼接了地址和路径参数，并且在后续使用时利用 `User{UID: "114514"}` 填入了参数。

接在在测试代码中使用了 `func req.Do(api req.API) (*http.Response, error)` 函数，它接收一个 `API` 并返回请求结果，我们使用命令 `go test -v -run ^TestUser$` 在屏幕上得到结果。

```
> go test -v -run ^TestUser$
=== RUN   TestUser
    req_test.go:31: {
          "args": {},
          "data": "",
          "files": {},
          "form": {},
          "headers": {
            "Accept-Encoding": "gzip",
            "Host": "httpbin.org",
            "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/116.0.0.0 Safari/537.36 Edg/116.0.1938.54"
          },
          "json": null,
          "method": "GET",
          "origin": "xxx.xxx.xxx.xxx",
          "url": "https://httpbin.org/anything/user/114514"
        }

--- PASS: TestUser (1.18s)
PASS
ok      github.com/Drelf2018/req/tests  1.438s
```

可以看到，代码成功发送了我们设置了路径参数为 `114514` 的 `GET` 请求。

### 带有 `Query` `Body` `Header` 参数的 `POST` 请求示例

```go
type Sign struct {
	req.PostForm
	UID           int    `api:"query"`
	Sign          string `api:"body"`
	RefreshNow    bool   `api:"body"`
	Authorization string `api:"header"`
}

func (Sign) RawURL() string {
	return "https://httpbin.org/post"
}

func TestSign(t *testing.T) {
	i, err := req.JSON(Sign{
		UID:           114514,
		Sign:          "逸一时误一世",
		RefreshNow:    true,
		Authorization: "Token 1919810",
	})
	if err != nil {
		t.Fatal(err)
	}

	b, err := json.MarshalIndent(i, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(string(b))
}
```

在上面代码中，我们定义了一个 `Sign` 结构体，并在其中嵌入了 `req.PostForm` 字段，这个字段与 `req.Get` 类似。

同时还添加了 `UID` `Sign` `RefreshNow` `Authorization` 四个字段，它们都带有 `api` 标签。这是本库定义的用来描述某个字段归属的标签，具体来说，这个标签目前支持 `query` `body` `header` `file` `files` 五个值，含义如其名。

接在在测试代码中使用了 `func req.JSON(api req.API) (any, error)` 函数，它接收一个 `API` 并返回请求结果经 `JSON` 反序列化的结果，我们使用命令 `go test -v -run ^TestSign$` 在屏幕上得到结果。

```
> go test -v -run ^TestSign$
=== RUN   TestSign
    req_test.go:61: {
          "args": {
            "uid": "114514"
          },
          "data": "",
          "files": {},
          "form": {
            "refresh_now": "true",
            "sign": "逸一时误一世"
          },
          "headers": {
            "Accept-Encoding": "gzip",
            "Authorization": "Token 1919810",
            "Content-Length": "76",
            "Content-Type": "application/x-www-form-urlencoded",
            "Host": "httpbin.org",
            "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/116.0.0.0 Safari/537.36 Edg/116.0.1938.54",
          },
          "json": null,
          "origin": "xxx.xxx.xxx.xxx",
          "url": "https://httpbin.org/post?uid=114514"
        }
--- PASS: TestSign (1.09s)
PASS
ok      github.com/Drelf2018/req/tests  1.314s
```

可以看到，代码成功发送了我们设置了 `Query` 中 `uid=114514`、`FormBody` 中 `sign=逸一时误一世` 和 `refresh_now=true`、`Header` 中 `Authorization=Token 1919810` 和 `Content-Type=application/x-www-form-urlencoded` 的 `POST` 请求。

### 参数名怎么来的

我们发现请求中参数名与字段名并不完全相同 `UID => uid` `RefreshNow => refresh_now` `Authorization => Authorization`

```go
var (
	nameReplacer   *strings.Replacer
	headerReplacer *strings.Replacer
)

func init() {
	oldnew1 := []string{"ID", "_id"}
	for i := 'A'; i <= 'Z'; i++ {
		oldnew1 = append(oldnew1, string(i)+"ID", "_"+string(i+32)+"id", string(i), "_"+string(i+32))
	}
	nameReplacer = strings.NewReplacer(oldnew1...)

	oldnew2 := make([]string, 0, 26*2)
	for i := 'A'; i <= 'Z'; i++ {
		oldnew2 = append(oldnew2, string(i), "-"+string(i))
	}
	headerReplacer = strings.NewReplacer(oldnew2...)
}

// 一般字段名替换器
func NameReplace(s string) string {
	return nameReplacer.Replace(s)[1:]
}

// 请求头字段名替换器
func HeaderReplace(s string) string {
	return headerReplacer.Replace(s)[1:]
}
```

这是因为在 [replacer.go](./replacer.go) 中定义了两种生成字段名的函数，其中 `HeaderReplace` 会用来替换所有带有 `api:"header"` 标签的字段名，剩下的由 `NameReplace` 处理。

通过阅读代码可以看出，`HeaderReplace` 会将所有大写字母 `X` 替换成 `-X` 这很符合请求头键名的规范，再由函数 `HeaderReplace` 做一个截取，去掉最前面的 `-` 。例如字段名 `ContentType => -Content-Type => Content-Type`

而 `NameReplace` 会将所有 `ID` 替换成 `_id` 或者形如 `XID` 的替换成 `_xid` 或者 `X` 替换成 `_x` ，再由函数去掉最前面的 `_` 。例如字段名 `UID => _uid => uid` `RefreshNow => _refresh_now => refresh_now`

```go
type Upload struct{
	HTML string `api:"body" req:"html"`
}
```

如果你对自动生成的字段名不满意，例如 `HTML => _h_t_m_l => h_t_m_l` ，可以使用 `req` 标签强制使用该名称。

### 参数值怎么来的

字段 `UID` 的类型是 `int` 为什么成功以字符串写入了 `query` 。`RefreshNow` 的类型是 `bool` 为什么成功以字符串写入了 `FormBody` ？

这是因为在 [marshal.go](./marshal.go) 中定义了 `func Marshal(i any) (string, error)` 函数。通过这个函数可以将任意常见类型转换成字符串。此外，对于实现了 `req.Marshaler` `json.Marshaler` 接口的值，也会被转换。最终，在没有任意一个类型匹配成功时，会直接对其进行 `JSON` 序列化得到字符串。

```go
func Marshal(i any) (string, error) {
	if i == nil {
		return "", nil
	}
	switch i := i.(type) {
	case Marshaler:
		return i.MarshalString()
	case json.Marshaler:
		b, err := i.MarshalJSON()
		return string(b), err
	case ...:
		// 省略部分类型 详见文件
	default:
		b, err := json.Marshal(i)
		return string(b), err
	}
}
```

## 拓展

### 参数 `Cookie` 怎么写入

在 [interfaces.go](./interfaces.go) 中定义了 `CookieJar` 接口。直接将实现了此接口的类型嵌入 `API` 中，并且在发起请求前为其赋值，程序会自动读取这个 `CookieJar` 。

```go
// 可判断有效性的 CookieJar
type CookieJar interface {
	IsValid() bool
	http.CookieJar
}
```

### 客户端 `Client`

之前在发送请求时使用了 `req.Do` 函数，实际上这个函数内部调用 `req.DefaultClient` 的 `func (c *Client) Do(api API) (*http.Response, error)` 方法。

```go
// 发送请求
func Do(api API) (*http.Response, error) {
	return DefaultClient.Do(api)
}
```

在 [client.go](./client.go) 中定义了客户端 `Client` 的结构体。

```go
// 客户端
type Client struct {
	http.Client
	BaseURL   *url.URL
	Header    http.Header
	Variables map[string]any
}
```

其中 `http.Client` 即每次发起请求时使用的客户端。

```go
// 这便是上面提到的自动添加 CookieJar 的实现
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
```

其中 `BaseURL` 即每次发起请求时使用基地址，需要使用的 `APICreator` 使用了这个方法。

```go
// 拼接 BaseURL 和提供的 rawURL
//
// 当 rawURL 以 "/" 开头时才会拼接
func (c *Client) URL(rawURL string) string {
	if c.BaseURL != nil && strings.HasPrefix(rawURL, "/") {
		rawURL = c.BaseURL.JoinPath(rawURL).String()
	}
	return rawURL
}
```

其中 `Header` 即每次发起请求时使用基础请求头，需要使用的 `APICreator` 使用了这个方法。

```go
// 向 req 请求添加请求头
//
// 会使用 Client 中设置的默认请求头
func (c *Client) AddHeader(req *http.Request, header []Field, val reflect.Value) (err error) {
	if c.Header != nil {
		req.Header = c.Header.Clone()
	}
	for _, data := range header {
		req.Header[data.Name] = []string{}  // 覆写基础请求头
		err = c.AddValue(req.Header, data, val)
		if err != nil {
			return
		}
	}
	return
}
```

其中 `Variables` 是一个用户可自行添加值的字典，它的用处在下面会介绍。

```go
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
```

### 为 `JSON` 格式响应体导出对应的结构体

在 [client.go](./client.go) 中有 `(*Client).Generate` 方法，它会请求这个 `API` 并将返回结果以 `JSON` 格式解析，如果成功会再将该 `JSON` 转成 `go` 语言的结构体形式，最后追加写入给定文件中。该功能为实验性功能，不多作介绍，如想了解详情请看 [converter.go](./converter.go) 源码。

```go
func TestGenerate(t *testing.T) {
	req.Generate("req_test.go", User{UID: "114514"})
}

// AutoGenerate ↓↓↓

type UserResponse struct {
	Args struct {
	} `json:"args"`
	Data  string `json:"data"` // ""
	Files struct {
	} `json:"files"`
	Form struct {
	} `json:"form"`
	Headers struct {
		AcceptEncoding string `json:"Accept-Encoding"` // "gzip"
		Host           string `json:"Host"`            // "httpbin.org"
		UserAgent      string `json:"User-Agent"`      // "Mozilla/5.1 (Windows NT 10.1; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/116.1.1.1 Safari/537.36 Edg/116.1.1938.54"
	} `json:"headers"`
	JSON   any    `json:"json"`
	Method string `json:"method"` // "GET"
	Origin string `json:"origin"` // "xxx.xxx.xxx.xxx"
	URL    string `json:"url"`    // "https://httpbin.org/anything/user/114514"
}

func GetUser() (result UserResponse, err error) {
	err = cli.Result(User{}, &result)
	return
}
```

### 标签 `api` 还能怎么用

其实，标签的完全格式为 `api:(query|body|header|file|files)[:value][,omitempty]`

当标签使用 `file` `files` 时，会对该字段类型进行特殊判断。采用 `file` 时会判断该字段是否实现了 `io.Reader` 接口，采用 `files` 时会判断该字段是否是列表或数组，并且元素的类型实现了 `io.Reader` 接口。本质是用来上传文件的，具体使用方法后面介绍。

标签后面的 `[:value]` `[,omitempty]` 这两个是互斥的，分别代表“当前字段值为空时要使用的默认值”和“当前字段值为空时忽略该字段”。字段判空使用的是 `func (v reflect.Value) IsZero() bool` 方法。

“忽略该字段”很好理解不做多解释，例如 `api:"query,omitempty"`

“默认值”指使用了类似 `api:"query:114"` `api:"body:$secret"` 标签时。如果“默认值”不以 `$` 开头，则会将该字符串直接写入这个参数值中。否则，会在 `Client` 中查找这个名称代表的值，如果没找到则会将带有 `$` 的这个字符串写入参数值。

```go
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
```

### 接口 `APICreator` 到底是个啥

直接从 [method.go](./method.go) 找出我写好了的一个文件上传请求的构造器，其中 `FileWriter` 定义在 [writer.go](./writer.go) 中。

```go
// 带有文件的请求体的 POST 请求构造器
type PostMultipartForm struct {
	FileWriter
}

func (PostMultipartForm) Method() string {
	return http.MethodPost
}

// 实现 APICreator 的方法 NewRequestWithContext 接收一个上下文 context.Context 一个客户端 *Client 以及一个 API 信息接口 APIData
func (p PostMultipartForm) NewRequestWithContext(ctx context.Context, cli *Client, api APIData) (req *http.Request, err error) {
	// 根据 API 加载任务
	task := LoadTask(api)
	// 获取 API 的值(reflect.Value)以便后续添加参数
	value := reflect.Indirect(reflect.ValueOf(api))
	// 判断当前有没有加载任意一种文件写入器
	if p.FileWriter == nil {
		// 使用默认的文件写入器 包装后的 *multipart.Writer
		p.FileWriter = &DefaultFileWriter{}
	}
	// 初始化写入器
	err = p.FileWriter.Initial()
	if err != nil {
		return
	}
	// 遍历 API 中的 file files 标签的字段
	var field reflect.Value
	for _, data := range task.Files {
		// 找到对应的值
		field, err = value.FieldByIndexErr(data.Index)
		if err != nil {
			return
		}
		// 为空直接跳过
		if field.IsZero() {
			continue
		}
		// 如果是 files 标签就说明有很多文件
		if field.Kind() == reflect.Slice || field.Kind() == reflect.Array {
			for i := 0; i < field.Len(); i++ {
				// 逐一写入文件写入器
				// 因为这些值在前在已经判断过是否实现 io.Reader 所以可以直接断言
				err = p.Write(field.Index(i).Interface().(io.Reader), data)
				if err != nil {
					return
				}
			}
		} else {
			// 如果是 file 标签就只写本身
			err = p.Write(field.Interface().(io.Reader), data)
			if err != nil {
				return
			}
		}
	}
	// 除了文件还要写一些常规的键值对
	for _, data := range task.Body {
		err = cli.AddValue(p, data, value)
		if err != nil {
			return
		}
	}
	// 关闭写入 等待读取 body
	err = p.Close()
	if err != nil {
		return
	}
	// 构建底层请求 *http.Request
	req, err = cli.AddBody(ctx, api, p.Reader())
	if err == nil {
		// 添加常规请求参数
		err = cli.AddQuery(req, task.Query, value)
		if err == nil {
			// 先添加请求头
			err = cli.AddHeader(req, task.Header, value)
			// 在对其覆写
			req.Header.Set("Content-Type", p.FormDataContentType())
		}
	}
	return
}

var _ APICreator = PostMultipartForm{}
```