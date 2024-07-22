package req_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/Drelf2018/req"
)

func TestJsonBody(t *testing.T) {
	body := make(req.JsonBody)
	body.Add("name", "nana7mi")
	body.Add("age", "17")

	buf := &bytes.Buffer{}
	err := json.NewEncoder(buf).Encode(body)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("buf.String(): %v\n", buf.String())
}
