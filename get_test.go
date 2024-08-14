package req_test

import (
	"net/url"
	"testing"

	"github.com/Drelf2018/req"
)

var client *req.Client

func init() {
	baseURL, err := url.Parse("https://httpbin.org/anything")
	if err != nil {
		panic(err)
	}
	client = &req.Client{BaseURL: baseURL}
	client.SetAuthorization("abc123")
	client.SetUserAgent(req.UserAgent)
}

func TestGetURL(t *testing.T) {
	httpbin := req.GetURL[map[string]any]("https://httpbin.org/get?q=1")
	data, err := httpbin.Do()

	if err != nil {
		t.Fatal(data, err)
	}
	t.Log(data)

	data, err = client.Debug(httpbin)
	if err != nil {
		t.Fatal(data, err)
	}
	t.Log(data)
}
