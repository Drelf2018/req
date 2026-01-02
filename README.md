<div align="center">

# req

_✨ 通过结构体发送请求 ✨_ 

</div>

## 如何实现一个 `API`

在正式开始使用前，我们需要了解如何实现一个 `API` 接口。

```go
// API 接口
type API interface {
	Method() string
	RawURL() string
}
```

在 [`interfaces.go`](interfaces.go) 中定义了 `API` 接口，它是用来描述请求的，包括请求地址 `RawURL` 方法和请求方法 `Method` 方法，通常需要用户自行实现。

```go
type SimpleGet struct{}

func (SimpleGet) Method() string {
	return http.MethodGet
}

func (SimpleGet) RawURL() string {
	return "https://httpbin.org/get"
}

func TestSimpleGet(t *testing.T) {
	text, err := req.Text(SimpleGet{})
	if err != nil {
		t.Fatal(err)
	}
	t.Log(text)
}
```

这样，你就实现了一个最小且可用的 `API` 结构体，并且使用默认会话发起请求。

## 为 `API` 添加参数

### 为 `GET` 请求添加路径参数和查询参数

路径参数可以简单的在 `RawURL` 方法中自行添加，而查询参数则需要通过本项目神奇的反射机制进行设置。

```go
type Get struct {
	UID       string
	PageLimit int `req:"query" default:"30"`
}

func (Get) Method() string {
	return http.MethodGet
}

func (g Get) RawURL() string {
	return fmt.Sprintf("https://httpbin.org/anything/followers/%s", g.UID)
}

func TestGet(t *testing.T) {
	text, err := req.Text(Get{UID: "10086", PageLimit: 10})
	if err != nil {
		t.Fatal(err)
	}
	t.Log(text)

	text, err = req.Text(Get{UID: "12306"})
	if err != nil {
		t.Fatal(err)
	}
	t.Log(text)
}
```

字段 `PageLimit` 后有两个标签，分别是 `req:"query"` 和 `default:"30"` 。前者用于声明这个字段要作为请求的 `query` 参数，后者表示该字段值为空时要使用的默认值。例如，测试函数中前后两次请求的地址分别为：

```
https://httpbin.org/anything/followers/10086?page_limit=10
```

```
https://httpbin.org/anything/followers/12306?page_limit=30
```

细心的朋友可能发现了，字段 `PageLimit` 的值自动设置在了地址中参数 `page_limit` 之后。这是因为项目 [`method/replacer.go`](method/replacer.go) 中内置了一个将驼峰字段名转换成下划线参数名的函数 `NameReplacer` 。你也可以自行替换这个函数变量，实现自己的参数名转换函数。

```go
var NameReplacer = CamelToSnake

// CamelToSnake 将字符串中的大写字母替换为下划线加小写字母，大写首字母前不添加下划线，连续的大写字母只在第一个字母前添加下划线
func CamelToSnake(s string) string {
	if s == "" {
		return ""
	}
	var result strings.Builder
	for i, r := range s {
		if unicode.IsUpper(r) {
			// 不是第一个字符并且前一个不是大写字母，添加下划线
			if i > 0 && !unicode.IsUpper(rune(s[i-1])) {
				result.WriteRune('_')
			}
			// 转成小写字母
			result.WriteRune(unicode.ToLower(r))
		} else {
			// 非大写字母直接添加
			result.WriteRune(r)
		}
	}
	return result.String()
}
```

### 标签 `req`

这个标签用于声明字段在请求中的参数类型，例如 `req:"body"` `req:"query"` 等。

其完整格式为 `req:"param[:name][,omitempty]"` ，一个具体的例子为 `req:"header:X-Forwarded-For,omitempty"` 。

`param` 区域是必填的，一般使用 `body` `query` `header` `cookie` 之一，具体原因可见后文。

`[:name]` 区域是可选的，如果对内置的参数名转换函数结果不满意，可以直接设置这个字段对应的参数名。

`[,omitempty]` 区域是可选的，当字段值是其类型零值并且选用了这个区域，那么就不会生成其对应的参数。

### 标签 `default`

这个标签用于设置字段值为其类型零值，并且没有在 `req` 标签中设置忽略时的默认值。

在项目 [`method/utils.go`](method/utils.go) 中有一个字符串转对应类型值的函数 `Unmarshal` ，可以把标签中设置的字符串转化成字段类型的值。

```go
// Unmarshal 将字符串转换为指定类型的值
func Unmarshal(value reflect.Value, s string) (any, error) {
	// ...
}
```

### 为 `POST` 请求添加请求体和请求头

请求体的设置与查询参数类似，在字段后使用标签 `req:"body"` 将其设置为请求体的键值对。请求体默认以 `JSON` 格式发送。

```go
type Post struct {
	UID            int    `req:"query"`
	Sign           string `req:"body"`
	RefreshNow     bool   `req:"body"`
	AcceptLanguage string `req:"header" default:"zh-CN"`
}

func (Post) Method() string {
	return http.MethodPost
}

func (Post) RawURL() string {
	return "https://httpbin.org/post"
}

func TestPost(t *testing.T) {
	text, err := req.Text(Post{UID: 10086, Sign: "这个人很懒", RefreshNow: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Log(text)
}
```

在上面代码中，我们定义了一个 `Post` 结构体。在发送请求后，服务端收到了我们的请求体：

```json
{
	"refresh_now": true,
	"sign": "\u8fd9\u4e2a\u4eba\u5f88\u61d2"
}
```

同时字段 `AcceptLanguage` 的值自动设置为了请求头 `Accept-Language` 的值。这同样是因为项目 [`method/replacer.go`](method/replacer.go) 中内置了一个将驼峰字段名转换成请求头的函数 `HeaderReplacer` 。

```go
var HeaderReplacer = CamelToHyphenated

// CamelToHyphenated 将字符串中的大写字母替换为横杠加大写字母，大写首字母前不添加横杠，连续的大写字母只在第一个字母前添加横杠
func CamelToHyphenated(s string) string {
	if s == "" {
		return ""
	}
	var result strings.Builder
	for i, r := range s {
		// 本身是大写字母，不是第一个字符并且前一个不是大写字母，添加横杠
		if unicode.IsUpper(r) && i > 0 && !unicode.IsUpper(rune(s[i-1])) {
			result.WriteRune('-')
		}
		result.WriteRune(r)
	}
	return result.String()
}
```

### 为请求添加 `Cookie`

`Cookie` 的设置与请求头类似，在字段后使用标签 `req:"cookie"` 将其设置为 `Cookie` 的键值对，或者让 `API` 实现 `http.CookieJar` 接口。

两者的区别在于请求返回 `Set-Cookie` 响应头字段时只会调用 `http.CookieJar` 接口的 `SetCookies` 方法，不会修改字段形式 `Cookie` 的值。

```go
type Cookie struct {
	UID       int    `req:"query"`
	SessionID string `req:"cookie"`
	token     string
}

func (Cookie) Method() string {
	return http.MethodGet
}

func (Cookie) RawURL() string {
	return "https://httpbin.org/get"
}

func (c *Cookie) SetCookies(u *url.URL, cookies []*http.Cookie) {
	for _, cookie := range cookies {
		if cookie.Name == "token" {
			c.token = cookie.Value
		}
	}
}

func (c *Cookie) Cookies(u *url.URL) []*http.Cookie {
	return []*http.Cookie{{Name: "token", Value: c.token}}
}

var _ http.CookieJar = (*Cookie)(nil)

func TestCookie(t *testing.T) {
	text, err := req.Text(&Cookie{UID: 10086, SessionID: "d926f241-28a4-4be3-8022-7b880b348bfa", token: "c99f18ad"})
	if err != nil {
		t.Fatal(err)
	}
	t.Log(text)
}
```

```
Cookie: session_id=d926f241-28a4-4be3-8022-7b880b348bfa; token=c99f18ad
```

### 为请求添加文件

`req.PostMultipartForm` 用于构造包含多部分表单数据的 `POST` 请求。它实现了 `APIBody` 接口和 `Method` 方法，可直接嵌入到自定义的 `API` 结构体中使用。类型为 `io.Reader` 的字段通过 `req:"body"` 标签声明后，会被作为文件内容上传。如果该字段实现了 `Name() string` 方法，会以此作为文件名，否则使用转换后的字段名。

```go
type Upload struct {
	req.PostMultipartForm

	Username string   `req:"body"`
	Avatar   *os.File `req:"body"`
}

func (Upload) RawURL() string {
	return "https://httpbin.org/post"
}

func TestUpload(t *testing.T) {
	file, err := os.Open("avatar.jpg")
	if err != nil {
		t.Fatal(err)
	}
	text, err := req.Text(Upload{
		Username: "Nana7mi",
		Avatar:   file,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Log(text)
}
```

## 利用 `API` 发送请求

之前在发送请求时使用了 `req.Text` 函数，实际上这个函数的内部调用了 `req.DefaultSession` 的 `Text` 方法。

```go
// 会话
type Session struct {
	http.Client

	// 基础路径，若 API 路径以 "/" 开头则会拼接在此路径后
	BaseURL *url.URL

	// 默认请求头，会自动为每个请求添加
	Header http.Header

	// 自定义变量，当字段标签 default 中的值以 "$" 开头则会尝试在该字典中查找对应值
	Variables map[string]any
}

var DefaultSession = &Session{
	Header: http.Header{
		"User-Agent": {UserAgent},
	},
}
```

这里的 `Session` 结构体是一个会话，提供了设置基础路径、设置默认请求头、设置自定义变量、拼接地址、创建请求、发送请求、请求前钩子、响应后钩子、获取字符串形式响应体、响应体写入文件、反序列化响应体至对象、解包错误等一系列功能。在 [`req.go`](req.go) 中提供的全局函数，都是调用默认会话的同名方法实现的。

### 自定义变量

在前文介绍标签 `default` 时，提到了它是用来设置默认值。如果要设置的默认值过于复杂、不适合在标签中以字符串形式表示，可以设置一个以 `$` 开头的变量名，并且使用会话的 `Set` 方法设置其值。

```go
type Variables struct {
	UID       int    `req:"query"`
	SessionID string `req:"cookie" default:"$sid"`
}

func (Variables) Method() string {
	return http.MethodGet
}

func (Variables) RawURL() string {
	return "https://httpbin.org/get"
}

func TestVariables(t *testing.T) {
	req.DefaultSession.Set("$sid", "d926f241-28a4-4be3-8022-7b880b348bfa")
	text, err := req.Text(&Variables{UID: 10086})
	if err != nil {
		t.Fatal(err)
	}
	t.Log(text)
}
```

```
Cookie: session_id=d926f241-28a4-4be3-8022-7b880b348bfa
```

如果不想使用全局默认值，可以使用需要传入上下文的函数。在查找默认值时，会优先调用上下文的 `Value` 方法，未找到时才会继续使用全局默认值。

```go
func TestVariables(t *testing.T) {
	req.DefaultSession.Set("$sid", "d926f241-28a4-4be3-8022-7b880b348bfa")
	ctx := context.WithValue(context.Background(), "$sid", "new-session-id")
	text, err := req.TextWithContext(ctx, &Variables{UID: 10086})
	if err != nil {
		t.Fatal(err)
	}
	t.Log(text)
}
```

```
Cookie: session_id=new-session-id
```

### 钩子

在 `API` 实现接口 `BeforeRequest` 后，会在发送请求前调用其方法，通常用于打印日志、修改请求参数、中断请求等。

```go
// 请求前钩子
type BeforeRequest interface {
	BeforeRequest(cli *http.Client, req *http.Request, api API) error
}
```

在 `API` 实现接口  `CheckResponse` 后，会在收到响应后调用其方法，通常用于判断**状态码**是否正确，当 `API` 未实现这个接口时，默认判断响应是否为 `200 OK` 。

```go
// 检验响应
type CheckResponse interface {
	CheckResponse(cli *http.Client, resp *http.Response, api API) error
}
```

### 解包错误

在 `result` 实现接口 `Unwrap` 后，会在调用 `Result` 方法时，对反序列化结果进行接口判断，通常用于判断**业务码**是否正确。

```go
// 可解包出错误的接口返回值
type Unwrap interface {
	Unwrap() error
}
```

```go
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
```

## 高级用法

### 通过 `API` 构造请求

在使用本项目的过程中，你有没有想过，结构体是怎么变成请求的？在 [`method/interfaces.go`](method/interfaces.go) 中定义了五种接口：

```go
// API Cookie
type APICookie interface {
	Cookie(r *http.Request, value reflect.Value, cookie []reflect.StructField) error
}

// API 请求体
type APIBody interface {
	Body(r *http.Request, value reflect.Value, body []reflect.StructField) (io.Reader, error)
}

// API 请求参数
type APIQuery interface {
	Query(r *http.Request, value reflect.Value, query []reflect.StructField) error
}

// API 请求头
type APIHeader interface {
	Header(r *http.Request, value reflect.Value, header []reflect.StructField) error
}

// API 自定义标签
type APICustom interface {
	Custom(r *http.Request, value reflect.Value, custom map[string][]reflect.StructField) error
}
```

你可以通过实现这些接口，来自行构建请求。例如本项目默认 `query` 的实现方法：

```go
// AddValue 根据提供的 StructField 添加对应的值
func AddValue(ctx context.Context, val reflect.Value, field reflect.StructField, add func(string, string))

// MakeURLValues 根据提供的 []StructField 制作 url.Values
func MakeURLValues(ctx context.Context, val reflect.Value, fields []reflect.StructField) url.Values {
	u := make(url.Values, len(fields))
	for _, field := range fields {
		AddValue(ctx, val, field, u.Add)
	}
	return u
}

// AddQuery 向 req 请求添加请求参数
func AddQuery(req *http.Request, val reflect.Value, query []reflect.StructField) {
	q := MakeURLValues(req.Context(), val, query)
	if len(q) != 0 {
		req.URL.RawQuery = q.Encode()
	}
}
```

上面代码中的全局函数均在 `method` 目录下导出，方便你用来构建自己的请求方式。

这里的 `ctx` 包装了用户传入的上下文，`val` 是传入的 `API` 对象的反射值，`query` 是**修改过的**设置了对应标签的结构体字段。

即传入 `APIBody` 接口的是带有 `req:"body"` 的字段，传入 `APIHeader` 的是带有 `req:"header"` 的字段等，前文提及的可以自定义的标签会传入 `APICustom` 接口。

以下是本项目的 `Form` 请求体的实现方法：

```go
// 以 Form 表单为请求体的 POST 请求构造器
type PostForm struct {
	ContentType string `req:"header" default:"application/x-www-form-urlencoded"`
}

func (PostForm) Method() string {
	return http.MethodPost
}

func (PostForm) Body(req *http.Request, value reflect.Value, body []reflect.StructField) (io.Reader, error) {
	return strings.NewReader(MakeURLValues(req.Context(), value, body).Encode()), nil
}

var _ APIBody = PostForm{}
```

注意到它还同时实现了 `Method` 方法并且包含一个 `ContentType` 请求头字段，这意味着你可以直接将它嵌入你的 `API` 结构体，本项目将四个已实现的构造器提升到了主目录。

```go
// req.go
type (
	Get               = method.Get
	PostJSON          = method.PostJSON
	PostForm          = method.PostForm
	PostMultipartForm = method.PostMultipartForm
)
```

```go
type Form struct {
	req.PostForm
	Age  int    `req:"body"`
	Name string `req:"body"`
}

func (Form) RawURL() string {
	return "https://httpbin.org/post"
}

func TestForm(t *testing.T) {
	text, err := req.Text(&Form{Age: 17, Name: "Nana7mi"})
	if err != nil {
		t.Fatal(err)
	}
	t.Log(text)
}
```

```
data: age=17&name=Nana7mi
```

### 重试计时器

在发送请求失败时，往往需要重复多次，均失败后才认定请求失败。但是我们不会无节制不停歇的重试，有没有好办法能让我们知道要重试几次、多久后重试呢？有的兄弟有的，在 [`retry.go`](retry.go) 中定义的 `RetryTicker` 接口可以轻松的结局这个问题。

```go
// RetryTicker 重试计时器
type RetryTicker interface {
	// NextRetry 用于计算下次重试前需要等待的时间，入参为已重试次数，返回值为等待时间以及是否继续重试
	NextRetry(retried int) (delay time.Duration, ok bool)
}
```

在这里预设了多种重试器方便使用和参考。

```go
// DoubleTicker 倍增计时器，初始重试间隔 1 秒，之后每次重试间隔翻倍，值为最大重试次数
type DoubleTicker int

func (t DoubleTicker) NextRetry(retried int) (time.Duration, bool) {
	return (1 << retried) * time.Second, retried < int(t)
}
```

### 上下文

在 [`value.go`](value.go) 里提供了 `WithMap` `WithValues` 两种可以携带值的上下文包裹函数。

### 请求池

在 [`pool.go`](pool.go) 里提供了 `Pool` 用于管理请求，它是零值可用的。