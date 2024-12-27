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

// GET 请求构造器
//
// 直接嵌入结构体即可使用
type Get struct{}

func (Get) Method() string {
	return http.MethodGet
}

// 可以通过这个方法学习如何自己实现一个构造器
func (Get) NewRequestWithContext(ctx context.Context, cli *Client, api APIData) (req *http.Request, err error) {
	// 因为是 GET 请求所以不添加 body
	req, err = cli.AddBody(ctx, api, nil)
	if err != nil {
		return
	}
	// 提取 API 中字段
	task := LoadTask(api)
	// 获取 API 的值(reflect.Value)以便后续添加参数
	value := reflect.Indirect(reflect.ValueOf(api))
	// 添加请求参数
	err = cli.AddQuery(req, task.Query, value)
	if err == nil {
		// 添加请求头
		err = cli.AddHeader(req, task.Header, value)
	}
	return
}

var _ APICreator = Get{}

// 以 JSON 为请求体的 POST 请求构造器
type PostJSON struct{}

func (PostJSON) Method() string {
	return http.MethodPost
}

func (PostJSON) NewRequestWithContext(ctx context.Context, cli *Client, api APIData) (req *http.Request, err error) {
	task := LoadTask(api)
	value := reflect.Indirect(reflect.ValueOf(api))
	m, err := cli.MakeJSONMap(task.Body, value)
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

// 以 Form 表单为请求体的 POST 请求构造器
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

// 带有文件的请求体的 POST 请求构造器
type PostMultipartForm struct {
	FileWriter
}

func (PostMultipartForm) Method() string {
	return http.MethodPost
}

// 实现 APICreator 的方法 NewRequestWithContext 接收一个上下文 context.Context 一个客户端 *Client 以及一个 API 信息接口 APIData
func (p PostMultipartForm) NewRequestWithContext(ctx context.Context, cli *Client, api APIData) (req *http.Request, err error) {
	// 根据 API 加载任务
	task := LoadTask(api)
	// 获取 API 的值(reflect.Value)以便后续添加参数
	value := reflect.Indirect(reflect.ValueOf(api))
	// 判断当前有没有加载任意一种文件写入器
	if p.FileWriter == nil {
		// 使用默认的文件写入器 封装后的 *multipart.Writer
		p.FileWriter = &DefaultFileWriter{}
	}
	// 初始化写入器
	err = p.FileWriter.Initial()
	if err != nil {
		return
	}
	// 遍历 API 中的 file files 标签的字段
	var field reflect.Value
	for _, data := range task.Files {
		// 找到对应的值
		field, err = value.FieldByIndexErr(data.Index)
		if err != nil {
			return
		}
		// 为空直接跳过
		if field.IsZero() {
			continue
		}
		// 如果是 files 标签就说明有很多文件
		if field.Kind() == reflect.Slice || field.Kind() == reflect.Array {
			for i := 0; i < field.Len(); i++ {
				// 逐一写入文件写入器
				// 因为这些值在前在已经判断过是否实现 io.Reader 所以可以直接断言
				err = p.Write(field.Index(i).Interface().(io.Reader), data)
				if err != nil {
					return
				}
			}
		} else {
			// 如果是 file 标签就只写本身
			err = p.Write(field.Interface().(io.Reader), data)
			if err != nil {
				return
			}
		}
	}
	// 除了文件还要写一些常规的键值对
	for _, data := range task.Body {
		err = cli.AddValue(p, data, value)
		if err != nil {
			return
		}
	}
	// 关闭写入 等待读取 body
	err = p.Close()
	if err != nil {
		return
	}
	// 构建底层请求 *http.Request
	req, err = cli.AddBody(ctx, api, p.Reader())
	if err == nil {
		// 添加常规请求参数
		err = cli.AddQuery(req, task.Query, value)
		if err == nil {
			// 先添加请求头
			err = cli.AddHeader(req, task.Header, value)
			// 在对其覆写
			req.Header.Set("Content-Type", p.FormDataContentType())
		}
	}
	return
}

var _ APICreator = PostMultipartForm{}
