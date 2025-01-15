package req

import (
	"testing"

	"github.com/Drelf2018/req"
)

type Status struct {
	req.Get

	MyCookie   string `api:"cookie"`
	YourCookie string `api:"cookie"`
}

func (Status) RawURL() string {
	return "https://httpbin.org/get"
}

type StatusResponse struct {
	Args struct {
	} `json:"args"`
	Headers struct {
		AcceptEncoding string `json:"Accept-Encoding"`
		Cookie         string `json:"Cookie"`
		Host           string `json:"Host"`
		UserAgent      string `json:"User-Agent"`
		XAmznTraceID   string `json:"X-Amzn-Trace-Id"`
	} `json:"headers"`
	Origin string `json:"origin"`
	URL    string `json:"url"`
}

func GetStatus() (StatusResponse, error) {
	return req.Result[StatusResponse](Status{MyCookie: "abc123", YourCookie: "xyz789"})
}

func TestRetry(t *testing.T) {
	r, err := GetStatus()
	if err != nil {
		c, cancel := req.WithRetry(req.DefaultRetryTicker)
		for range c {
			r, err = GetStatus()
			if err == nil {
				cancel()
			}
		}
	}
	if err != nil {
		t.Fatal(err, r)
	}
	t.Log(r)
}
