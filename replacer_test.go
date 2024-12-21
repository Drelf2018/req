package req_test

import (
	"testing"

	"github.com/Drelf2018/req"
)

func TestReplace(t *testing.T) {
	assert := func(fn func(string) string, name, val string) {
		s := fn(name)
		if s != val {
			t.Fatalf("replace error: %s[want: %s get: %s]", name, val, s)
		}
	}
	assert(req.HeaderReplace, "AcceptLanguage", "Accept-Language")
	assert(req.NameReplace, "Mode", "mode")
	assert(req.NameReplace, "PostJson", "post_json")
	assert(req.NameReplace, "ID", "id")              // not i_d
	assert(req.NameReplace, "ReplyMID", "reply_mid") // not reply_m_id
	assert(req.NameReplace, "CSS", "post_json")
}
