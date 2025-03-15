package req_test

import (
	"testing"
	"time"

	"github.com/Drelf2018/req"
)

type Cookie struct {
	req.Get

	MyCookie   string `api:"cookie"`
	YourCookie string `api:"cookie"`
}

func (Cookie) RawURL() string {
	return "https://httpbin.org/get"
}

type CookieResponse struct {
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

func GetCookie() (CookieResponse, error) {
	return req.Result[CookieResponse](Cookie{MyCookie: "abc123", YourCookie: "xyz789"})
}

func TestRetry(t *testing.T) {
	r, err := GetCookie()
	if err != nil {
		c, cancel := req.WithRetry(req.DefaultRetryTicker)
		for range c {
			r, err = GetCookie()
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

func TestFibonacci(t *testing.T) {
	now := time.Now()
	c, cancel := req.WithRetry(req.FibonacciTicker{time.Second, 2 * time.Second})
	for i := range c {
		n := time.Now()
		t.Log(i, n.Sub(now))
		now = n
		if i == 4 {
			cancel()
		}
	}
}
