package template

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"text/template"

	"github.com/PuerkitoBio/goquery"
	"github.com/tidwall/gjson"
)

// EnvPrefix 环境变量简写前缀
var EnvPrefix string = "$env."

// TrimEnvPrefix 修剪环境变量前缀
func TrimEnvPrefix(s string) (string, bool) {
	if t := strings.TrimSpace(s); strings.HasPrefix(t, EnvPrefix) {
		return strings.TrimPrefix(t, EnvPrefix), true
	}
	return s, false
}

// ToBuffer 将模板转为缓冲区
func ToBuffer(tmpl *template.Template, text string, data any) (*bytes.Buffer, error) {
	tmpl, err := tmpl.Parse(text)
	if err != nil {
		return nil, err
	}
	buf := &bytes.Buffer{}
	err = tmpl.Execute(buf, data)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

// ToString 将模板转为字符串
func ToString(tmpl *template.Template, text string, data any, getter ...*OrderedMap) (string, error) {
	if len(getter) != 0 {
		if text, ok := TrimEnvPrefix(text); ok {
			v, err := Getter.Get(getter, text)
			if err != nil {
				return "", err
			}
			s, ok := v.(string)
			if !ok {
				return "", fmt.Errorf("invalid getter.Get(%q) type: expected string, got: %T", text, v)
			}
			return s, nil
		}
	}
	buf, err := ToBuffer(tmpl, text, data)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

// Bind 在模板上绑定环境变量函数
func Bind(tmpl *template.Template, setter *OrderedMap, getter ...*OrderedMap) {
	tmpl.Funcs(template.FuncMap{"env": func(key string, value ...any) (any, error) {
		switch len(value) {
		case 0:
			return Getter.Get(getter, key)
		case 1:
			setter.Set(key, value[0])
			return "", nil
		default:
			return nil, fmt.Errorf("too many arguments: expected 1 or 2, got: %d", 1+len(value))
		}
	}})
}

// Range 遍历更新模板
func Range(tmpl *template.Template, data any, ranger, setter *OrderedMap, getter ...*OrderedMap) error {
	var tmplErr TemplateError
	Bind(tmpl, setter, getter...)
	ranger.Iterate(func(key string, value any) bool {
		v, ok := value.(string)
		// 不是字符串，不需要进一步解析，直接设置值
		if !ok {
			setter.Set(key, value)
			return true
		}
		// 值有前缀，先获取其真实值，再设置
		if v, ok = TrimEnvPrefix(v); ok {
			value, err := Getter.Get(getter, v)
			if err != nil {
				tmplErr.Add(key, err)
			} else {
				setter.Set(key, value)
			}
			return true
		}
		// 键有前缀，需要将值嵌套在环境变量设置模板中，因为模板本身有设置值的功能，直接解析即可
		if s, ok := TrimEnvPrefix(key); ok {
			_, err := ToString(tmpl, fmt.Sprintf(`{{ env "%s" (%s) }}`, s, v), data)
			if err != nil {
				tmplErr.Add(key, err)
			}
			return true
		}
		// 直接解析成字符串后设置值
		v, err := ToString(tmpl, v, data)
		if err != nil {
			tmplErr.Add(key, err)
		} else {
			setter.Set(key, v)
		}
		return true
	})
	return tmplErr.Unwrap()
}

// Plaintext 获取纯净文本
func Plaintext(text string) (string, error) {
	html := strings.NewReader(text)
	doc, err := goquery.NewDocumentFromReader(html)
	if err != nil {
		return "", err
	}
	doc.Find("img").Each(func(i int, s *goquery.Selection) {
		if alt, ok := s.Attr("alt"); ok {
			s.SetText(alt)
		}
	})
	return doc.Text(), nil
}

// BuiltinFuncMap 内置函数
var BuiltinFuncMap = template.FuncMap{
	"int": strconv.Atoi,
	"str": strconv.Itoa,
	"reg": func(pattern string, s string) (string, error) {
		expr, err := regexp.Compile(pattern)
		if err != nil {
			return "", err
		}
		return expr.FindString(s), nil
	},
	"type": func(v any) string {
		return fmt.Sprintf("%T", v)
	},
	"json": func(v any) (string, error) {
		b, err := json.Marshal(v)
		return string(b), err
	},
	"gjson": func(path, json string) (any, error) {
		if r := gjson.Get(json, path); r.Exists() {
			return r.Value(), nil
		}
		return nil, fmt.Errorf("path not found: %q", path)
	},
	"default": func(def, val any) any {
		if val != nil {
			return val
		}
		return def
	},
	"base64encode": func(s string) string {
		return base64.StdEncoding.EncodeToString([]byte(s))
	},
	"base64decode": func(s string) (string, error) {
		data, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			return "", err
		}
		return string(data), nil
	},
	"plaintext": Plaintext,
}

func NewTemplate(name string) *template.Template {
	return template.New(name).Funcs(BuiltinFuncMap)
}
