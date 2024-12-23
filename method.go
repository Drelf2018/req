package req

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
)

type Get struct{}

func (Get) Method() string {
	return http.MethodGet
}

func (Get) NewRequestWithContext(ctx context.Context, cli *Client, api APIData) (req *http.Request, err error) {
	req, err = cli.AddBody(ctx, api, nil)
	if err != nil {
		return
	}
	task := LoadTask(api)
	value := reflect.Indirect(reflect.ValueOf(api))
	err = cli.AddQuery(req, task.Query, value)
	if err == nil {
		err = cli.AddHeader(req, task.Header, value)
	}
	return
}

var _ APICreator = Get{}

type PostJSON struct{}

func (PostJSON) Method() string {
	return http.MethodPost
}

func (PostJSON) NewRequestWithContext(ctx context.Context, cli *Client, api APIData) (req *http.Request, err error) {
	task := LoadTask(api)
	value := reflect.Indirect(reflect.ValueOf(api))
	m, err := cli.JSONMap(task.Body, value)
	if err != nil {
		return
	}
	buf := &bytes.Buffer{}
	err = json.NewEncoder(buf).Encode(m)
	if err != nil {
		return
	}
	buf.Truncate(buf.Len() - 1) // Seeing the source code of (*json.Encoder).Encode

	req, err = cli.AddBody(ctx, api, buf)
	if err == nil {
		err = cli.AddQuery(req, task.Query, value)
		if err == nil {
			err = cli.AddHeader(req, task.Header, value)
		}
	}
	return
}

var _ APICreator = PostJSON{}

type PostForm struct{}

func (PostForm) Method() string {
	return http.MethodPost
}

func (PostForm) NewRequestWithContext(ctx context.Context, cli *Client, api APIData) (req *http.Request, err error) {
	task := LoadTask(api)
	value := reflect.Indirect(reflect.ValueOf(api))

	form := make(url.Values)
	for _, data := range task.Body {
		err = cli.AddValue(form, data, value)
		if err != nil {
			return
		}
	}

	req, err = cli.AddBody(ctx, api, strings.NewReader(form.Encode()))
	if err == nil {
		err = cli.AddQuery(req, task.Query, value)
		if err == nil {
			err = cli.AddHeader(req, task.Header, value)
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
	}
	return
}

var _ APICreator = PostForm{}

type PostMultipartForm struct {
	FileWriter
}

func (PostMultipartForm) Method() string {
	return http.MethodPost
}

func (p PostMultipartForm) NewRequestWithContext(ctx context.Context, cli *Client, api APIData) (req *http.Request, err error) {
	task := LoadTask(api)
	value := reflect.Indirect(reflect.ValueOf(api))
	if p.FileWriter == nil {
		p.FileWriter = &DefaultFileWriter{}
	}
	err = p.FileWriter.Initial()
	if err != nil {
		return
	}

	var field reflect.Value
	for _, data := range task.Files {
		field, err = value.FieldByIndexErr(data.Index)
		if err != nil {
			return
		}
		if field.IsZero() {
			continue
		}
		if field.Kind() == reflect.Slice || field.Kind() == reflect.Array {
			for i := 0; i < field.Len(); i++ {
				err = p.Write(field.Index(i).Interface().(io.Reader), data)
				if err != nil {
					return
				}
			}
		} else {
			err = p.Write(field.Interface().(io.Reader), data)
			if err != nil {
				return
			}
		}
	}
	for _, data := range task.Body {
		err = cli.AddValue(p, data, value)
		if err != nil {
			return
		}
	}
	err = p.Close()
	if err != nil {
		return
	}

	req, err = cli.AddBody(ctx, api, p.Reader())
	if err == nil {
		err = cli.AddQuery(req, task.Query, value)
		if err == nil {
			err = cli.AddHeader(req, task.Header, value)
			req.Header.Set("Content-Type", p.FormDataContentType())
		}
	}
	return
}

var _ APICreator = PostMultipartForm{}
