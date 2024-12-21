package req_test

import (
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

var _ req.Marshaler = (*reqMarshal)(nil)

type user struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func TestMarshal(t *testing.T) {
	for k, v := range map[any]string{
		jsonMarshal{}:       "jsonMarshal",
		reqMarshal{}:        "reqMarshal",
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
