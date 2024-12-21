package req

import (
	"bytes"
	"io"
	"mime/multipart"
	"path/filepath"
)

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

type FileWriter interface {
	Adder
	io.Closer
	Initial() error
	Reader() io.Reader
	FormDataContentType() string
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

func (w *DefaultFileWriter) Add(key, val string) {
	w.Writer.WriteField(key, val)
}

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
