package req_test

import (
	"os"
	"testing"

	"github.com/Drelf2018/req"
)

func TestDebug(t *testing.T) {
	api := sendDanmaku{UID: 12138, List: []string{"1", "1", "4"}}
	api.File, _ = os.Open("go.mod")

	data, err := req.Debug(api)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(data)
}
