package req

import (
	"bytes"
	"io"
	"mime/multipart"
	"path/filepath"
)

// 向多部份写入器写入文件
func WriteFormFile(w *multipart.Writer, fieldname string, filename string, file io.Reader) error {
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

// 文件写入器
type FileWriter interface {
	Adder
	io.Closer

	// 初始化函数 可以做一些赋值操作
	Initial() error

	// 获取最终请求体
	Reader() io.Reader

	// 请求头 Content-Type
	FormDataContentType() string

	// 写入一个文件
	Write(file io.Reader, data Field) error
}

type DefaultFileWriter struct {
	*multipart.Writer
	buf *bytes.Buffer
}

func (w *DefaultFileWriter) Initial() error {
	w.buf = &bytes.Buffer{}
	w.Writer = multipart.NewWriter(w.buf)
	return nil
}

// 添加普通键值对
func (w *DefaultFileWriter) Add(key, val string) {
	w.Writer.WriteField(key, val)
}

// 写入文件
func (w *DefaultFileWriter) Write(file io.Reader, data Field) error {
	switch file := file.(type) {
	case NamedReader:
		return WriteFormFile(w.Writer, filepath.Join(data.Name, file.Name()), file.Name(), file)
	default:
		return WriteFormFile(w.Writer, filepath.Join(data.Name, data.Value), data.Value, file)
	}
}

func (w *DefaultFileWriter) Reader() io.Reader {
	return w.buf
}

var _ FileWriter = &DefaultFileWriter{}

func NewFileWriter() *DefaultFileWriter {
	buf := &bytes.Buffer{}
	return &DefaultFileWriter{multipart.NewWriter(buf), buf}
}
