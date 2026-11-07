<div align="center">

# req

_✨ 通过结构体发送请求 ✨_ 

</div>

## 如何实现一个 `API`

在正式开始使用前，我们需要了解如何实现一个 `API` 接口。

```go
// API 用于定义基本接口
type API interface {
	Method() string
	RawURL() string
}
```

在 [`method/interfaces.go`](method/interfaces.go) 中定义了 `API` 接口，它是用来描述请求的，包括请求方法 `Method` 方法和请求地址 `RawURL` 方法，通常需要用户自行实现。

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

细心的朋友可能发现了，字段 `PageLimit` 的值自动设置在了地址中参数 `page_limit` 之后。这是因为项目 [`method/task.go`](method/task.go) 中内置了一个参数名转换器 `Renamer` ，它会按参数类型分派：`header` 目标转成横杠连接，其它目标转成下划线连接。你也可以替换 `DefaultRenamer` 变量，实现自己的参数名转换。

```go
// Renamer 参数名转换器
type Renamer struct{}

func (Renamer) Rename(target string, name string) string {
	if target == "header" {
		return PascalToHyphenated(name)
	}
	return PascalToSnake(name)
}

// DefaultRenamer 全局参数名转换器
var DefaultRenamer APIRenamer = Renamer{}
```

### 标签 `req`

这个标签用于声明字段在请求中的参数类型，例如 `req:"body"` `req:"query"` 等。

其完整格式为 `req:"<target>[:<name>][,omitempty|,required]"` ，一个具体的例子为 `req:"header:X-Forwarded-For,omitempty"` 。

`<target>` 区域是必填的，一般使用 `body` `query` `header` `cookie` 之一。也可以写自定义标签，交给 `APICustom` 钩子处理。

`[:<name>]` 区域是可选的，如果对内置的参数名转换结果不满意，可以直接设置这个字段对应的参数名。

修饰符区域是可选的，二者互斥，且都不能和 `default` 标签同时使用：

- `,omitempty` 整条取值链都没取到值时，不生成这个参数。
- `,required` 整条取值链都没取到值时，直接返回错误 `method.ErrMissing` 。

### 标签 `default`

这个标签用于设置字段值为其类型零值，并且没有在 `req` 标签中设置忽略时的默认值。

默认值会按字段类型解码：字符串目标（`query` `header` `cookie`）原样发出。`body` 目标则先转成字段类型，例如 `int` 字段写 `default:"30"` 得到的是数字 `30` 而不是字符串 `"30"` 。这一步由 [`method/codec.go`](method/codec.go) 里的 `DecodeType` 完成。

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

需要留意的是，这个构造器在构建请求体时会把文件字段读完并顺手 `Close()` ，所以它构建的请求**不能重试**。

## 利用 `API` 发送请求

之前在发送请求时使用了 `req.Text` 函数，实际上这个函数的内部调用了 `req.DefaultSession` 的 `Text` 方法。

```go
// Session 会话
type Session struct {
	http.Client

	// 基础路径，若 API 路径以 "/" 开头则会拼接在此路径后
	BaseURL *url.URL

	// 默认请求头，会自动为每个请求添加
	Header http.Header
}

// DefaultSession 默认会话
var DefaultSession = &Session{
	Header: http.Header{
		"User-Agent": {UserAgent},
	},
}
```

这里的 `Session` 结构体是一个会话，提供了拼接地址 `JoinPath`、创建请求 `NewRequest`、发送请求 `Do`、克隆自己 `Clone`、获取响应体 `Content` `Text`、写入文件 `Write`、反序列化 `Result` `JSON` 等功能。在 [`req.go`](req.go) 中提供的全局函数，都是调用默认会话的同名方法实现的。

会话的默认配置不在方法里，而是由 `NewSession` 和一组选项函数给出：

```go
session := req.NewSession(
	req.SetClient(http.DefaultClient),
	req.ParseURL("https://httpbin.org"),
	req.SetHeader("Accept-Language", "zh-CN"),
	req.SetAuthorization("Bearer 123456"),
	req.SetUserAgent(req.UserAgent),
)
```

可用的选项有 `SetClient` `ParseURL` `SetHeaders` `SetHeaderMap` `SetHeader` `SetAuthorization` `SetUserAgent` 。其中 `SetHeaders` 会整体替换默认请求头，`SetHeaderMap` 和 `SetHeader` 则是添加。

每个发送请求的方法都有一个带 `WithContext` 的版本，例如 `Text` 和 `TextWithContext` ，实现超时控制或取消。

```go
ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
defer cancel()
text, err := session.TextWithContext(ctx, MyAPI{})
```

### 取值链

取值链按固定顺序往下找，**命中就停**：

| 顺序 | 来源 | 声明方式 | 拿到的值 |
|---|---|---|---|
| 1 | 字段自身 | —— | 字段的实际值，但 `IsZero()` 为假才算有值 |
| 2 | 上下文 | `ctx:"<key>"` | `ctx.Value(key)` |
| 3 | Cookie | `cookie:"<name>"` | `CookieReader.Cookie(name)` |
| 4 | 环境变量 | `env:"<key>"` | `os.LookupEnv(key)` |
| 5 | 默认值 | `default:"<value>"` | 值本身 |

整条链走完还是没值，再看 `req` 标签上的修饰符：

| 链尾策略 | 行为 |
|---|---|
| `,omitempty` | 这个键**整个不出现** |
| `,required` | 报错 `req/method: required but no value resolved` |
| 都没有 | 标量用字段类型的**零值**，切片和映射则**整个不出现** |

```go
type Resolve struct {
	// 1. 字段自身
	UID int `req:"query"`

	// 2. 值来自上下文，写进 query 时叫 token
	Token string `req:"query:token" ctx:"token"`

	// 3. 值来自请求里的 session_id，写进请求头时叫 X-Session
	Session string `req:"header:X-Session" cookie:"session_id"`

	// 4. 值来自环境变量
	Region string `req:"query" env:"REGION"`

	// 5. 值来自默认值
	PageLimit int `req:"query" default:"30"`

	// 链尾：没值就整个省略
	Debug string `req:"query,omitempty"`

	// 链尾：没值就报错
	Must string `req:"query,required"`
}
```

后三步（Cookie / 环境变量 / 默认值）拿到的都是**字符串**。只有 JSON 请求体这类**保留类型**的去处才会按字段类型解码（`"30"` → `int`），query / header / cookie 始终按字符串发送。上下文的取值是 `any`，**类型原样保留**，写进 JSON 请求体时不会变成字符串。

下面这个例子把"值从哪来"和"值去哪"分开用：字段本身没值，值从上下文的 `sid` 拿，最后写成一个名叫 `session_id` 的 Cookie。

```go
type FromContext struct {
	UID       int    `req:"query"`
	SessionID string `req:"cookie:session_id" ctx:"sid"`
}

func (FromContext) Method() string {
	return http.MethodGet
}

func (FromContext) RawURL() string {
	return "https://httpbin.org/get"
}

func TestFromContext(t *testing.T) {
	ctx := context.WithValue(context.Background(), "sid", "d926f241-28a4-4be3-8022-7b880b348bfa")
	text, err := req.TextWithContext(ctx, &FromContext{UID: 10086})
	if err != nil {
		t.Fatal(err)
	}
	t.Log(text)
}
```

```
Cookie: session_id=d926f241-28a4-4be3-8022-7b880b348bfa
```

### 钩子

一次请求里所有钩子的调用顺序是固定的：

```
解析字段 → cookie 钩子 → header 钩子 → body 钩子 → 建请求
        → query 钩子 → custom 钩子 → BeforeRequest → 发送 → AfterResponse
```

其中 `BeforeRequest` 和 `AfterResponse` 在**每次重试**时都会重新跑一遍。

```go
// BeforeRequest 请求前钩子
type BeforeRequest interface {
	BeforeRequest(*http.Client, *http.Request, API) error
}
```

在 `API` 实现接口 `AfterResponse` 后，会在收到响应后调用其方法，通常用于判断**状态码**是否正确。当 `API` 未实现这个接口时，默认要求响应状态码在 `2xx` 范围内。

```go
// AfterResponse 响应后钩子，用于检验状态码，出现错误时必须自行关闭响应体
type AfterResponse interface {
	AfterResponse(*http.Client, *http.Response, API) error
}
```

### 解包错误

在 `result` 实现接口 `Unwrap` 后，会在调用 `Result` 方法时，对反序列化结果进行接口判断，通常用于判断**业务码**是否正确。

```go
// Unwrap 用于从响应体中解包出错误
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
// APIHeader 用于设置请求头
type APIHeader interface {
	Header(ctx context.Context, api API, reader CookieReader, header []Field) (http.Header, error)
}

// APICookie 用于设置 Cookie
type APICookie interface {
	Cookie(ctx context.Context, api API, reader CookieReader, cookie []Field) ([]*http.Cookie, error)
}

// APIBody 用于设置请求体
type APIBody interface {
	Body(ctx context.Context, api API, reader CookieReader, body []Field) (io.Reader, http.Header, error)
}

// APIQuery 用于设置请求参数
type APIQuery interface {
	Query(ctx context.Context, api API, reader CookieReader, query []Field) (string, error)
}

// APICustom 用于处理自定义标签
type APICustom interface {
	Custom(ctx context.Context, api API, r *http.Request, custom map[string][]Field) error
}
```

它们的形状是统一的「环境 + 载荷」：`ctx` 和 `CookieReader` 是环境，`[]Field` 是载荷。钩子只负责把值**做出来返回**，不直接改请求。

`Field` 里已经带好了编译期算出来的编码器，所以钩子拿到的是现成的字段描述，不需要自己反射。

`CookieReader` 是一个只有一个方法的接口：

```go
// CookieReader 用于读取 Cookie
type CookieReader interface {
	Cookie(name string) (*http.Cookie, error)
}
```

你可以通过实现这些接口，来自行构建请求。例如本项目默认 `query` 的实现方法：

```go
// WalkFields 按取值链逐条产出字段的名字和值，按字段顺序、切片逐元素展开
func WalkFields(ctx context.Context, api API, reader CookieReader, fields []Field, yield func(name, value string)) error {
	value := reflect.Indirect(reflect.ValueOf(api))
	for _, field := range fields {
		ss, err := field.ToString(ctx, value, reader)
		if err != nil {
			return err
		}
		for _, s := range ss {
			yield(field.Name, s)
		}
	}
	return nil
}

// MakeURLValues 按取值链把字段做成 url.Values，切片会展开成多条同名参数
func MakeURLValues(ctx context.Context, api API, reader CookieReader, fields []Field) (url.Values, error) {
	u := make(url.Values, len(fields))
	err := WalkFields(ctx, api, reader, fields, u.Add)
	if err != nil {
		return nil, err
	}
	return u, nil
}

// MakeQuery 按取值链把 query 字段做成参数字符串
func MakeQuery(ctx context.Context, api API, reader CookieReader, query []Field) (string, error) {
	q, err := MakeURLValues(ctx, api, reader, query)
	if err != nil {
		return "", err
	}
	return q.Encode(), nil
}
```

上面代码中的全局函数均在 `method` 目录下导出，方便你用来构建自己的请求方式。

这里的 `ctx` 是用户传入的上下文，`api` 是 `API` 对象本身，`reader` 是用来获取 Cookie 的载体，`query` 是设置了 `req:"query"` 的字段。

以下是本项目的 `Form` 请求体的实现方法：

```go
// PostForm 是 POST 请求构造器，以 Form 表单创建请求体
type PostForm struct{}

func (PostForm) Method() string {
	return http.MethodPost
}

func (PostForm) Body(ctx context.Context, api API, reader CookieReader, body []Field) (io.Reader, http.Header, error) {
	form, err := MakeURLValues(ctx, api, reader, body)
	if err != nil {
		return nil, nil, err
	}
	return strings.NewReader(form.Encode()), ContentType("application/x-www-form-urlencoded"), nil
}

var _ APIBody = PostForm{}
```

注意到它还同时实现了 `Method` 方法，所以你可以直接把它嵌入自己的 `API` 结构体。响应体的 Content-Type 由 `Body` 的第二个返回值给出。

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

### 延迟器

在发送请求失败时，往往需要重复多次，均失败后才认定请求失败。但是我们不会无节制不停歇的重试，有没有好办法能让我们知道要重试几次、多久后重试呢？有的兄弟有的，在 [`delay.go`](delay.go) 中定义的 `Delayer` 接口可以轻松地解决这个问题。

```go
// Delayer 延迟器，决定重试的间隔与次数。
//
// 向延迟器传入已重试次数，会判断是否继续重试并计算休眠间隔。
type Delayer interface {
	NextDelay(times int) (delay time.Duration, ok bool)
}
```

在这里预设了多种延迟器方便使用和参考。

```go
// DoubleDelayer 倍增延迟器，初始延迟间隔 1 秒，之后每次延迟间隔翻倍，值为最大延迟次数
type DoubleDelayer int

func (d DoubleDelayer) NextDelay(times int) (time.Duration, bool) {
	return (1 << times) * time.Second, times < int(d)
}

// DefaultDelayer 默认延迟器，“试”不过三
var DefaultDelayer Delayer = DoubleDelayer(2)
```

| 延迟器 | 行为 |
|---|---|
| `DoubleDelayer(n)` | 从 1 秒开始每次翻倍，最多重试 `n` 次，`DefaultDelayer` 就是 `DoubleDelayer(2)` |
| `ForeverDelayer(d)` | 永远返回 `d`，**永远不到期** |
| `ZeroDelayer(n, whySafe)` | 零间隔，最多重试 `n` 次；`whySafe` 是逼你写清为什么不怕打爆对方 |
| `FibonacciDelayer{1s, 2s, 10s}` | 前两项是种子，之后按斐波那契递增，超过第三项就停 |
| `RandomDelayer{1s, 5s}` | 在区间里随机，**永远不到期** |
| `DelayFunc(f)` | 把 `func(int) (time.Duration, bool)` 包成延迟器 |

把它用到请求上很简单，嵌入已写好的延迟器，或者给 `API` 加一个 `NextDelay` 方法，会话会在第一次失败之后开始重试。

```go
func (x *MyAPI) NextDelay(times int) (time.Duration, bool) {
	return req.DefaultDelayer.NextDelay(times)
}
```

- `times` 是**已经重试过的次数**（从 0 开始）。
- 返回 `ok == false`（延迟器到期）就停止重试，并把**最后一次**请求的错误返回出去。
- 每次重试都会**重新构建请求**，重新跑一遍取值链，也重新跑一遍 `BeforeRequest`。
- 等待期间 `ctx` 被取消会立刻中止，并返回 `ctx.Err()`。

### Cookie 转换器

在 [`cookie/cookie.go`](cookie/cookie.go) 里提供了可以将结构体中 `string` 类型或嵌入的结构体中 `string` 类型的字段转换成对应 `*http.Cookie` 对象的函数。只有设置了 `cookie` 标签的导出字段会被用于存取。想兜住那些结构体没声明的键值，可以加一个 `map[string]string` 字段并标上 `cookie:",embed"` 。另外注意嵌入的结构体**必须是导出的**，否则整块会被跳过。

```go
type UserInfo struct {
	Name        string `cookie:"name"`
	Description string `cookie:"Desc"`
}

type Cookies struct {
	Token     string `cookie:"Token"`
	SessionID string `cookie:"session_id"`
	Temp      string
	UserInfo
}

func TestCookies(t *testing.T) {
	c := Cookies{
		Token:     "c99f18ad",
		SessionID: "d926f241-28a4-4be3-8022-7b880b348bfa",
		Temp:      "temp",
		UserInfo: UserInfo{
			Name:        "Nana7mi",
			Description: "Shark",
		},
	}
	t.Log(cookie.Get(&c))
}
```

```
[Token=c99f18ad session_id=d926f241-28a4-4be3-8022-7b880b348bfa name=Nana7mi Desc=Shark] <nil>
```

### 保活 Cookie

[`cookie/keepalive.go`](cookie/keepalive.go) 里的 `KeepaliveCookieJar` 把任意 `http.CookieJar` 包起来，只负责**验证一次**：

- 实现 `Refresher`（`Refresh`）就能用；再实现 `Validator`（`Valid`）就会先查、无效才刷，否则每次直接刷新。
- `State()` 读当前状态，`SinceVerified()` 看距离上次验证成功过了多久。
- `Logger` 字段用于记录检验 `http.CookieJar` 时出现的错误。

````go
k := &cookie.KeepaliveCookieJar{CookieJar: jar, Refresher: r}

ctx, cancel := context.WithCancel(context.Background())
defer cancel()

k.Verify(ctx) // 想先验一次就放这里
ticker := req.NewDelayTicker(req.ForeverDelayer(time.Minute))
defer ticker.Stop()
for {
	select {
	case <-ctx.Done():
		return
	case _, ok := <-ticker.C:
		if !ok {
			return
		}
		_ = k.Verify(ctx)
	}
}
````

### 工作流引擎

在 [`template`](template) 目录下提供了一个工作流引擎，用于通过 YAML 编排一系列请求：并发任务调度、`needs` 依赖、`if` 条件（success/failure/cancelled）、`curl` 风格步骤、输入校验、输出提取、依赖环检测一应俱全，还会自动管理步骤间的 Cookie 会话。反正我是用来实现 `webhook` 的，你想知道怎么用的话请参考 [`template/README.md`](template/README.md) 。
