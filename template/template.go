package template

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"text/template"

	"github.com/PuerkitoBio/goquery"
	"github.com/tidwall/gjson"
)

// ToValue 将模板转为真实值
func ToValue[T any](tmpl *template.Template, text string, data any) (value T, err error) {
	if tmpl == nil {
		tmpl = NewTemplate("")
	} else {
		tmpl, err = tmpl.Clone()
		if err != nil {
			return
		}
	}
	tmpl.Funcs(template.FuncMap{"save": func(v T) string {
		value = v
		return ""
	}})
	tmpl, err = tmpl.Parse(fmt.Sprintf("{{ %s | save }}", text))
	if err != nil {
		return
	}
	err = tmpl.Execute(io.Discard, data)
	return
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
