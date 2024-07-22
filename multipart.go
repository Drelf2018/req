package req

import (
	"bytes"
	"io"
	"mime/multipart"
)

type Writer multipart.Writer

func (w *Writer) Add(fieldname string, value string) {
	_ = (*multipart.Writer)(w).WriteField(fieldname, value)
}

func (w *Writer) WriteFile(fieldname string, filename string, file io.Reader) error {
	writer, err := (*multipart.Writer)(w).CreateFormFile(fieldname, filename)
	if err != nil {
		return err
	}

	_, err = io.Copy(writer, file)
	if err != nil {
		return err
	}

	if closer, ok := file.(io.Closer); ok {
		if err = closer.Close(); err != nil {
			return err
		}
	}
	return nil
}

func (w *Writer) Close() error {
	return (*multipart.Writer)(w).Close()
}

// call Close before use ContentType!
func (w *Writer) ContentType() string {
	return (*multipart.Writer)(w).FormDataContentType()
}

func NewPipe() (*Writer, io.Reader) {
	buf := &bytes.Buffer{}
	return (*Writer)(multipart.NewWriter(buf)), buf
}
