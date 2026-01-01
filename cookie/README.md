<div align="center">

# req/cookie

_✨ 通过结构体管理 Cookie ✨_ 

</div>

## Cookie 转换器

在 [`cookie.go`](cookie.go) 里提供了可以将结构体中 `string` 类型或嵌入的结构体中 `string` 类型的字段转换成对应 `*http.Cookie` 对象的函数。只有设置了 `cookie` 标签的导出字段会被用于存取。

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
	t.Log(cookie.Get(c))
}
```

```
[session_id=d926f241-28a4-4be3-8022-7b880b348bfa name=Nana7mi Desc=Shark Token=c99f18ad]
```

## 可持续化 Cookie 池

在 [`pool.go`](pool.go) 里提供了一个池，用来持续化保存、刷新、获取 `http.CookieJar` ，具体怎么用我也还没搞清楚，就当留给读者的课后题吧！