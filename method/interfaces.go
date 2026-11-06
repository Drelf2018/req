package method

import (
	"context"
	"io"
	"net/http"
)

// Marshaler 用于将对象序列化为字符串
type Marshaler interface {
	MarshalString() (string, error)
}

// Unmarshaler 用于将字符串反序列化进对象
type Unmarshaler interface {
	UnmarshalString(string) error
}

// CookieReader 用于读取 Cookie
type CookieReader interface {
	Cookie(name string) (*http.Cookie, error)
}

// API 用于定义基本接口
type API interface {
	Method() string
	RawURL() string
}

// APIRenamer 用于重命名字段
type APIRenamer interface {
	Rename(target string, name string) string
}

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
