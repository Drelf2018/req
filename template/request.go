package template

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/textproto"
	"reflect"
	"strings"
	"text/template"

	"github.com/Drelf2018/req"
	"github.com/Drelf2018/req/method"
)

// Request 用模板、步骤和数据发送请求
type Request struct {
	Tmpl   *template.Template
	Data   any
	Step   *Step
	Getter Getter
}

func (r *Request) replace(target any) (err error) {
	switch target := target.(type) {
	case map[string]any:
		for key, value := range target {
			if v, ok := value.(string); ok {
				if v, ok = TrimEnvPrefix(v); ok {
					target[key], err = r.Getter.Get(v)
				} else {
					target[key], err = ToString(r.Tmpl, v, r.Data)
				}
			} else {
				err = r.replace(value)
			}
			if err != nil {
				return
			}
		}
	case []any:
		for i, value := range target {
			if v, ok := value.(string); ok {
				if v, ok = TrimEnvPrefix(v); ok {
					target[i], err = r.Getter.Get(v)
				} else {
					target[i], err = ToString(r.Tmpl, v, r.Data)
				}
			} else {
				err = r.replace(value)
			}
			if err != nil {
				return
			}
		}
	}
	return nil
}

func (r *Request) Method() string {
	return r.Step.Method
}

func (r *Request) RawURL() string {
	return r.Step.URL
}

var _ req.API = (*Request)(nil)

func (r *Request) Cookie(req *http.Request, _ reflect.Value, _ []reflect.StructField) (err error) {
	if r.Step.Cookie == nil {
		return
	}
	if v, ok := r.Step.Cookie.(string); ok {
		if v, ok = TrimEnvPrefix(v); ok {
			r.Step.Cookie, err = r.Getter.Get(v)
			if err != nil {
				return
			}
		}
	}
	switch cookie := r.Step.Cookie.(type) {
	case string:
		cookie, err = ToString(r.Tmpl, cookie, r.Data)
		if err != nil {
			return
		}
		parts := strings.Split(textproto.TrimString(cookie), ";")
		if len(parts) == 1 && parts[0] == "" {
			return
		}
		for _, pair := range parts {
			pair = textproto.TrimString(pair)
			name, value, found := strings.Cut(pair, "=")
			if !found {
				return fmt.Errorf("req/template: invalid cookie pair: %q", pair)
			}
			req.AddCookie(&http.Cookie{Name: name, Value: value})
		}
		return
	case map[string]any:
		for name, value := range cookie {
			v, ok := value.(string)
			if !ok {
				return fmt.Errorf("req/template: invalid cookie[%q] type: %T", name, value)
			}
			v, err = ToString(r.Tmpl, v, r.Data, r.Getter...)
			if err != nil {
				return
			}
			req.AddCookie(&http.Cookie{Name: name, Value: v})
		}
		return
	default:
		return fmt.Errorf("req/template: invalid cookie type: %T", r.Step.Cookie)
	}
}

var _ method.APICookie = (*Request)(nil)

func (r *Request) Body(*http.Request, reflect.Value, []reflect.StructField) (io.Reader, error) {
	if r.Step.Body == nil {
		return nil, nil
	}
	if v, ok := r.Step.Body.(string); ok {
		if v, ok = TrimEnvPrefix(v); ok {
			var err error
			r.Step.Body, err = r.Getter.Get(v)
			if err != nil {
				return nil, err
			}
		}
	}
	switch body := r.Step.Body.(type) {
	case string:
		if body == "" {
			return nil, nil
		}
		return ToBuffer(r.Tmpl, body, r.Data)
	case map[string]any, []any:
		err := r.replace(body)
		if err != nil {
			return nil, err
		}
		buf := &bytes.Buffer{}
		err = json.NewEncoder(buf).Encode(body)
		if err != nil {
			return nil, err
		}
		buf.Truncate(buf.Len() - 1) // Seeing the source code of (*json.Encoder).Encode
		return buf, nil
	default:
		return nil, fmt.Errorf("req/template: invalid body type: %T", r.Step.Body)
	}
}

var _ method.APIBody = (*Request)(nil)

func (r *Request) Header(req *http.Request, _ reflect.Value, _ []reflect.StructField) error {
	if r.Step.Header == nil {
		return nil
	}
	err := r.replace(r.Step.Header)
	if err != nil {
		return err
	}
	for key, value := range r.Step.Header {
		switch vs := value.(type) {
		case string:
			req.Header.Set(key, vs)
		case []any:
			for _, v := range vs {
				if v, ok := v.(string); ok {
					req.Header.Add(key, v)
				}
			}
		default:
			return fmt.Errorf("req/template: invalid header[%q] type: %T", key, value)
		}
	}
	return nil
}

var _ method.APIHeader = (*Request)(nil)
