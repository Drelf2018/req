package req_test

import (
	"io"
	"testing"

	"github.com/Drelf2018/req"
)

type jsonMarshal struct{}

func (jsonMarshal) MarshalJSON() ([]byte, error) {
	return []byte("jsonMarshal"), nil
}

type reqMarshal struct{}

func (reqMarshal) MarshalString() (string, error) {
	return "reqMarshal", nil
}

type reader struct{}

func (reader) Read(p []byte) (n int, err error) {
	return copy(p, "reader"), io.EOF
}

type stringer struct{}

func (stringer) String() string {
	return "stringer"
}

type goStringer struct{}

func (goStringer) GoString() string {
	return "goStringer"
}

type user struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func TestMarshal(t *testing.T) {
	for k, v := range map[any]string{
		jsonMarshal{}:       "jsonMarshal",
		reqMarshal{}:        "reqMarshal",
		reader{}:            "reader",
		stringer{}:          "stringer",
		goStringer{}:        "goStringer",
		"str":               "str",
		true:                "true",
		false:               "false",
		-1:                  "-1",
		3.14:                "3.14",
		user{"nana7mi", 17}: `{"name":"nana7mi","age":17}`,
	} {
		s, err := req.Marshal(k)
		if err != nil {
			t.Fatal(err)
		}
		if s != v {
			t.Fatal("want:", v, "get:", s)
		}
	}
}
