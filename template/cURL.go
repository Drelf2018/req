package template

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"

	"github.com/mattn/go-shellwords"
)

// argsToRequest 将 cURL 字符串的参数解析为 *http.Request
func argsToRequest(ctx context.Context, args ...string) (*http.Request, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("empty curl arguments")
	}

	var (
		rawURL    string
		method    string
		getMethod bool
		body      []string
		headers   []string
		cookies   []string
		user      string
		referer   string
		userAgent string
	)

	for i := 0; i < len(args); i++ {
		switch t := args[i]; t {
		case "-X", "--request":
			i++
			if i < len(args) {
				method = strings.ToUpper(args[i])
			}
		case "-G", "--get":
			getMethod = true
		case "-d", "--data", "--data-raw", "--data-ascii", "--data-binary", "--data-urlencode":
			i++
			if i < len(args) {
				body = append(body, strings.Split(args[i], "&")...)
			}
		case "-H", "--header":
			i++
			if i < len(args) {
				headers = append(headers, args[i])
			}
		case "-b", "--cookie":
			i++
			if i < len(args) && strings.Contains(args[i], "=") {
				cookies = append(cookies, args[i])
			}
		case "-u", "--user":
			i++
			if i < len(args) {
				user = args[i]
			}
		case "-e", "--referer":
			i++
			if i < len(args) {
				referer = args[i]
			}
		case "-A", "--user-agent":
			i++
			if i < len(args) {
				userAgent = args[i]
			}
		default:
			switch {
			case strings.HasPrefix(t, "-X"):
				method = strings.ToUpper(t[2:])
			case strings.HasPrefix(t, "http://") || strings.HasPrefix(t, "https://") || strings.Contains(t, "://") || strings.Contains(t, "/"):
				rawURL = t
			default:
				return nil, fmt.Errorf("invalid curl argument %q", t)
			}
		}
	}

	if rawURL == "" {
		return nil, fmt.Errorf("no URL found in curl arguments")
	}

	if getMethod && len(body) != 0 {
		u, err := url.Parse(rawURL)
		if err != nil {
			return nil, fmt.Errorf("invalid URL %q: %w", rawURL, err)
		}
		q := u.Query()
		for _, query := range body {
			key, value, _ := strings.Cut(query, "=")
			q.Add(strings.TrimSpace(key), strings.TrimSpace(value))
		}
		u.RawQuery = q.Encode()
		rawURL = u.String()
		body = nil
	}

	var bodyReader io.Reader
	if len(body) != 0 {
		bodyReader = strings.NewReader(strings.Join(body, "&"))
	}

	if method == "" {
		if len(body) == 0 || getMethod {
			method = http.MethodGet
		} else {
			method = http.MethodPost
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, rawURL, bodyReader)
	if err != nil {
		return nil, err
	}

	if len(cookies) != 0 {
		req.Header.Set("Cookie", strings.Join(cookies, ";"))
	}

	if username, password, ok := strings.Cut(user, ":"); ok {
		req.SetBasicAuth(username, password)
	}

	if referer != "" {
		req.Header.Set("Referer", referer)
	}

	if userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	}

	for _, header := range headers {
		if k, v, ok := strings.Cut(header, ":"); ok {
			req.Header.Add(strings.TrimSpace(k), strings.TrimSpace(v))
		}
	}

	// 如果 body 是 application/x-www-form-urlencoded，且没有显式设置 Content-Type，自动补上
	if len(body) != 0 && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	return req, nil
}

// ArgsToRequest 将 cURL 字符串的参数解析为 *http.Request
func ArgsToRequest(ctx context.Context, args ...string) (*http.Request, error) {
	req, err := argsToRequest(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("req/template.ArgsToRequest: %w", err)
	}
	return req, nil
}

// CURLToRequest 将 cURL 字符串解析为 *http.Request
func CURLToRequest(ctx context.Context, curl string) (*http.Request, error) {
	args, err := shellwords.Parse(curl)
	if err != nil {
		return nil, fmt.Errorf("req/template.CURLToRequest: failed to parse curl: %w", err)
	}
	req, err := argsToRequest(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("req/template.CURLToRequest: %w", err)
	}
	return req, nil
}

// curlAPI 将 argsToRequest 的产物桥接回根库会话
type curlAPI struct {
	req  *http.Request
	body string // 反读出的请求体文本
}

func (c curlAPI) Method() string { return c.req.Method }
func (c curlAPI) RawURL() string { return c.req.URL.String() }

// 实现 method.APIBody：Session 重建请求时塞回 body
func (c curlAPI) Body(*http.Request, reflect.Value, []reflect.StructField) (io.Reader, error) {
	if c.body == "" {
		return nil, nil
	}
	return strings.NewReader(c.body), nil
}

// 实现 method.APIHeader：curl 的显式头覆盖会话默认头
func (c curlAPI) Header(r *http.Request, _ reflect.Value, _ []reflect.StructField) error {
	for k, vs := range c.req.Header {
		r.Header.Del(k) // 先清掉默认头，让 curl 显式值优先
		for _, v := range vs {
			r.Header.Add(k, v)
		}
	}
	return nil
}

// 实现 method.APICookie：curl -b 的显式 Cookie
func (c curlAPI) Cookie(r *http.Request, _ reflect.Value, _ []reflect.StructField) error {
	for _, ck := range c.req.Cookies() {
		r.AddCookie(ck)
	}
	return nil
}
