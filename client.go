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
	http.Client
	BaseURL string
	Header  http.Header
	Default map[string]any
}

func (c *Client) add(adder Adder, data Field, v reflect.Value) error {
	field, err := v.FieldByIndexErr(data.Index)
	if err != nil {
		return err
	}

	if field.IsZero() {
		if !data.Omit {
			if strings.HasPrefix(data.Value, "$") {
				s, err := Marshal(c.Default[data.Value])
				if err != nil {
					return err
				}
				adder.Add(data.Key, s)
			} else {
				adder.Add(data.Key, data.Value)
			}
		}
		return nil
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
	var (
		v           = reflect.ValueOf(api)
		task        = LoadTask(api)
		body        io.Reader
		ContentType string
	)
	// data
	if task.Files != nil {
		buf := &bytes.Buffer{}
		w := multipart.NewWriter(buf)

		for _, data := range task.Files {
			field, errNil := v.FieldByIndexErr(data.Index)
			if errNil != nil {
				return nil, errNil
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
			field, errNil := v.FieldByIndexErr(data.Index)
			if errNil != nil {
				return nil, errNil
			}
			if field.IsZero() {
				if !data.Omit {
					if strings.HasPrefix(data.Value, "$") {
						s, err = Marshal(c.Default[data.Value])
						if err != nil {
							return
						}
					} else {
						s = data.Value
					}
					err = w.WriteField(data.Key, s)
					if err != nil {
						return
					}
				}
				continue
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
		ContentType = w.FormDataContentType()
	} else if task.Body != nil {
		if _, isJson := api.(postJson); isJson {
			m := make(map[string]any)
			for _, data := range task.Body {
				field, errNil := v.FieldByIndexErr(data.Index)
				if errNil != nil {
					return nil, errNil
				}
				if field.IsZero() {
					if !data.Omit {
						if strings.HasPrefix(data.Value, "$") {
							m[data.Key] = c.Default[data.Value]
						} else {
							m[data.Key] = data.Value
						}
					}
				} else {
					m[data.Key] = field.Interface()
				}
			}

			buf := &bytes.Buffer{}
			err = json.NewEncoder(buf).Encode(m)
			if err != nil {
				return
			}

			body = buf
			ContentType = "application/json"
		} else {
			values := make(url.Values)
			for _, data := range task.Body {
				err = c.add(values, data, v)
				if err != nil {
					return
				}
			}

			body = strings.NewReader(values.Encode())
			if _, isForm := api.(postForm); isForm {
				ContentType = "application/x-www-form-urlencoded"
			}
		}
	}
	// new request
	req, err = http.NewRequestWithContext(ctx, api.Method(), c.BaseURL+api.URL(), body)
	if err != nil {
		return
	}
	// query
	if task.Query != nil {
		query := make(url.Values)
		for _, data := range task.Query {
			err = c.add(query, data, v)
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
	req.Header.Set("User-Agent", UserAgent)
	if ContentType != "" {
		req.Header.Set("Content-Type", ContentType)
	}
	for _, data := range task.Header {
		err = c.add(req.Header, data, v)
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

var DefaultClient = &Client{}
