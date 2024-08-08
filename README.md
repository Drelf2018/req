# req

通过结构体保存发送请求所需字段

### 使用

```go
type Color string

func (c Color) MarshalString() (string, error) {
	i, err := strconv.ParseInt(strings.ReplaceAll(string(c), "#", ""), 16, 64)
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(i, 10), nil
}

var _ req.Marshaler = (*Color)(nil)

type sendDanmaku struct {
	req.PostJson
	UID            int      `api:"query"`
	Mode           string   `api:"body;1"`
	Msg            string   `api:"body"`
	Color          Color    `api:"body;16777215"`
	ReplyMID       int      `api:"body,omitempty"`
	List           []string `api:"body"`
	File           *os.File `api:"files"`
	AcceptLanguage string   `api:"header"`
}

func (sendDanmaku) URL() string {
	return "https://httpbin.org/post"
}

func TestDebug(t *testing.T) {
	api := sendDanmaku{UID: 12138, List: []string{"1", "1", "4"}, Color: "#FFFFFF", AcceptLanguage: "zh-CN"}
	api.File, _ = os.Open("go.mod")
	data, err := req.Debug(api)
	if err != nil {
		t.Fatal(data, err)
	}
	t.Log(data)
}
```

#### 控制台

```cmd
=== RUN   TestDebug
    response_test.go:46: map[args:map[uid:12138] data: files:map[file:module github.com/Drelf2018/req

        go 1.18
        ] form:map[color:16777215 list:[1 1 4] mode:1 msg:] headers:map[Accept-Encoding:gzip Accept-Language:zh-CN Content-Length:969 Content-Type:multipart/form-data; 
boundary=0ba4e248ea89a4772faebbe5fea7f87a6e70f6ba788b57d98d691a35c100 Host:httpbin.org User-Agent:Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/116.0.0.0 Safari/537.36 Edg/116.0.1938.54 X-Amzn-Trace-Id:Root=1-66b47ef8-513f4c60256f50b674299064] json:<nil> url:https://httpbin.org/post?uid=12138]
--- PASS: TestDebug (1.15s)
```