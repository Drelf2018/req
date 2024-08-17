package req

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"reflect"
	"strings"
)

const Omitempty string = "omitempty"
const UserAgent string = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/116.0.0.0 Safari/537.36 Edg/116.0.1938.54"

func WriteFile(w *multipart.Writer, fieldname string, filename string, file io.Reader) error {
	writer, err := w.CreateFormFile(fieldname, filename)
	if err != nil {
		return err
	}

	_, err = io.Copy(writer, file)
	if err != nil {
		return err
	}

	if closer, ok := file.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

type Client struct {
	Client  http.Client
	BaseURL *url.URL
	Header  http.Header

	// Client will use the value in Variables when the Field's Value starts with "$"
	Variables map[string]any
}

func (c *Client) value(key string) any {
	if c.Variables == nil {
		return nil
	}
	if strings.HasPrefix(key, "$") {
		return c.Variables[key]
	}
	return nil
}

func (c *Client) valueString(key string) (string, error) {
	i := c.value(key)
	if i != nil {
		return Marshal(i)
	}
	return key, nil
}

func (c *Client) SetAuthorization(auth string) {
	if c.Header == nil {
		c.Header = make(http.Header)
	}
	c.Header.Set("Authorization", auth)
}

func (c *Client) Authorization() (auth string) {
	if c.Header == nil {
		return ""
	}
	return c.Header.Get("Authorization")
}

func (c *Client) SetUserAgent(val string) {
	if c.Header == nil {
		c.Header = make(http.Header)
	}
	c.Header.Set("User-Agent", val)
}

func (c *Client) UserAgent() string {
	if c.Header == nil {
		return ""
	}
	return c.Header.Get("User-Agent")
}

func (c *Client) add(adder Adder, data Field, v reflect.Value) error {
	field, err := v.FieldByIndexErr(data.Index)
	if err != nil {
		return err
	}

	if field.IsZero() {
		if data.Omit {
			return nil
		}
		if data.Value != "" {
			s, err := c.valueString(data.Value)
			if err != nil {
				return err
			}
			adder.Add(data.Key, s)
			return nil
		}
	}

	switch field.Kind() {
	case reflect.Array, reflect.Slice:
		for i := 0; i < field.Len(); i++ {
			s, err := Marshal(field.Index(i).Interface())
			if err != nil {
				return err
			}
			adder.Add(data.Key, s)
		}
	default:
		s, err := Marshal(field.Interface())
		if err != nil {
			return err
		}
		adder.Add(data.Key, s)
	}

	return nil
}

func (c *Client) NewRequestWithContext(ctx context.Context, api Api) (req *http.Request, err error) {
	// initial
	if i, isBefore := api.(BeforeRequest); isBefore {
		err = i.BeforeRequest(ctx, c)
		if err != nil {
			return
		}
	}
	// load task
	val := reflect.Indirect(reflect.ValueOf(api))
	task := LoadTask(api)
	var (
		body        io.Reader
		contentType string
	)
	// data
	if task.Files != nil {
		buf := &bytes.Buffer{}
		w := multipart.NewWriter(buf)

		var field reflect.Value
		for _, data := range task.Files {
			field, err = val.FieldByIndexErr(data.Index)
			if err != nil {
				return
			}
			if field.IsZero() {
				continue
			}
			switch file := field.Interface().(type) {
			case NamedReader:
				err = WriteFile(w, data.Key, file.Name(), file)
			case io.Reader:
				err = WriteFile(w, data.Key, data.Value, file)
			}
			if err != nil {
				return
			}
		}

		var s string
		for _, data := range task.Body {
			field, err = val.FieldByIndexErr(data.Index)
			if err != nil {
				return
			}
			if field.IsZero() {
				if data.Omit {
					continue
				}
				if data.Value != "" {
					s, err = c.valueString(data.Value)
					if err != nil {
						return
					}
					err = w.WriteField(data.Key, s)
					if err != nil {
						return
					}
					continue
				}
			}
			switch field.Kind() {
			case reflect.Array, reflect.Slice:
				for i := 0; i < field.Len(); i++ {
					s, err = Marshal(field.Index(i).Interface())
					if err != nil {
						return
					}
					err = w.WriteField(data.Key, s)
					if err != nil {
						return
					}
				}
			default:
				s, err = Marshal(field.Interface())
				if err != nil {
					return
				}
				err = w.WriteField(data.Key, s)
				if err != nil {
					return
				}
			}
		}

		err = w.Close()
		if err != nil {
			return
		}
		body = buf
		contentType = w.FormDataContentType()
	} else if task.Body != nil {
		if _, isJson := api.(postJson); isJson {
			m := make(map[string]any)

			var field reflect.Value
			for _, data := range task.Body {
				field, err = val.FieldByIndexErr(data.Index)
				if err != nil {
					return
				}
				if field.IsZero() {
					if data.Omit {
						continue
					}
					if data.Value != "" {
						i := c.value(data.Value)
						if i != nil {
							m[data.Key] = i
						} else {
							m[data.Key] = data.Value
						}
						continue
					}
				}
				m[data.Key] = field.Interface()
			}

			buf := &bytes.Buffer{}
			err = json.NewEncoder(buf).Encode(m)
			if err != nil {
				return
			}
			buf.Truncate(buf.Len() - 1) // See the source code of (*json.Encoder).Encode
			body = buf
			contentType = "application/json"
		} else {
			values := make(url.Values)
			for _, data := range task.Body {
				err = c.add(values, data, val)
				if err != nil {
					return
				}
			}

			body = strings.NewReader(values.Encode())
			if _, isForm := api.(postForm); isForm {
				contentType = "application/x-www-form-urlencoded"
			}
		}
	}
	// new request
	u := api.URL()
	if c.BaseURL != nil && strings.HasPrefix(u, "/") {
		req, err = http.NewRequestWithContext(ctx, api.Method(), c.BaseURL.JoinPath(u).String(), body)
	} else {
		req, err = http.NewRequestWithContext(ctx, api.Method(), u, body)
	}
	if err != nil {
		return
	}
	// query
	if task.Query != nil {
		query := make(url.Values)
		for _, data := range task.Query {
			err = c.add(query, data, val)
			if err != nil {
				return
			}
		}
		req.URL.RawQuery = query.Encode()
	}
	// header
	if c.Header != nil {
		req.Header = c.Header.Clone()
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for _, data := range task.Header {
		err = c.add(req.Header, data, val)
		if err != nil {
			return
		}
	}
	return
}

func (c *Client) NewRequest(api Api) (req *http.Request, err error) {
	return c.NewRequestWithContext(context.Background(), api)
}

// result must be a pointer!
func (c *Client) DoWithContext(ctx context.Context, api Api, result any) (err error) {
	req, err := c.NewRequestWithContext(ctx, api)
	if err != nil {
		return
	}

	resp, err := c.Client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(result)
	if err != nil {
		return
	}

	if i, ok := result.(Unwrap); ok {
		err = i.Unwrap()
	}
	return
}

// result must be a pointer!
func (c *Client) Do(api Api, result any) (err error) {
	return c.DoWithContext(context.Background(), api, result)
}

func (c *Client) Debug(api Api) (m map[string]any, err error) {
	m = make(map[string]any)
	err = c.Do(api, &m)
	return
}

var DefaultClient = &Client{
	Header: http.Header{
		"User-Agent": {UserAgent},
	},
}
