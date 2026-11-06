package method

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"unicode"
)

// PascalToSnake 将字符串中的大写字母替换为下划线加小写字母，大写首字母前不添加下划线，连续的大写字母只在第一个字母前添加下划线
func PascalToSnake(s string) string {
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

// PascalToHyphenated 将字符串中的大写字母替换为横杠加大写字母，大写首字母前不添加横杠，连续的大写字母只在第一个字母前添加横杠
func PascalToHyphenated(s string) string {
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

// ContentType 用于快捷创建 Content-Type 请求头
func ContentType(ct string) http.Header {
	return http.Header{"Content-Type": []string{ct}}
}

// NewJSONReader 将提供的值序列化后存入 *bytes.Buffer
func NewJSONReader(v any) (io.Reader, error) {
	buf := &bytes.Buffer{}
	err := json.NewEncoder(buf).Encode(v)
	if err != nil {
		return nil, fmt.Errorf("req/method: cannot marshal %T: %w", v, err)
	}
	buf.Truncate(buf.Len() - 1) // Seeing the source code of (*json.Encoder).Encode
	return buf, nil
}

// MakeMap 按取值链把 body 字段做成 map
func MakeMap(ctx context.Context, api API, reader CookieReader, body []Field) (map[string]any, error) {
	value := reflect.Indirect(reflect.ValueOf(api))
	m := make(map[string]any, len(body))
	for _, field := range body {
		v, err := field.ToValue(ctx, value, reader)
		if err != nil {
			return nil, err
		}
		if v != nil {
			m[field.Name] = v
		}
	}
	return m, nil
}

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

// MakeCookies 按取值链把 cookie 字段做成 Cookie 切片，保持字段顺序
func MakeCookies(ctx context.Context, api API, reader CookieReader, cookie []Field) ([]*http.Cookie, error) {
	cookies := make([]*http.Cookie, 0, len(cookie))
	err := WalkFields(ctx, api, reader, cookie, func(name, value string) {
		cookies = append(cookies, &http.Cookie{Name: name, Value: value})
	})
	if err != nil {
		return nil, err
	}
	return cookies, nil
}

// MakeHeader 按取值链把 header 字段做成请求头映射
func MakeHeader(ctx context.Context, api API, reader CookieReader, header []Field) (http.Header, error) {
	h := make(http.Header, len(header))
	err := WalkFields(ctx, api, reader, header, h.Add)
	if err != nil {
		return nil, err
	}
	return h, nil
}

// MakeQuery 按取值链把 query 字段做成参数字符串
func MakeQuery(ctx context.Context, api API, reader CookieReader, query []Field) (string, error) {
	q, err := MakeURLValues(ctx, api, reader, query)
	if err != nil {
		return "", err
	}
	return q.Encode(), nil
}
