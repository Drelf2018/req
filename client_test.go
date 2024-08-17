package req_test

import (
	"net/http"
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
	client = &req.Client{BaseURL: baseURL, Variables: map[string]any{
		"$appid":  "10086",
		"$secret": "...",
	}}
	client.SetAuthorization("abc123")
	client.SetUserAgent(req.UserAgent)
}

// A minimum req.Api instance
type TestApi string

func (TestApi) Method() string {
	return http.MethodGet
}

func (t TestApi) URL() string {
	return string(t)
}

func TestClient(t *testing.T) {
	// url starts with "/": use BaseURL
	m, err := client.Debug(TestApi("/baseurl"))
	if err != nil {
		t.Fatal(m, err)
	}
	t.Log("request 1:", m["url"]) // "https://httpbin.org/anything/baseurl"

	// url does not have a "/" prefix: BaseURL is not used
	m, err = client.Debug(TestApi("https://httpbin.org/get?q=1"))
	if err != nil {
		t.Fatal(m, err)
	}
	t.Log("request 2:", m["url"]) // "https://httpbin.org/get?q=1"
}

func TestCURL(t *testing.T) {
	s, err := client.CURL(TestApi("https://httpbin.org/get?q=1"))
	if err != nil {
		t.Fatal(err)
	}
	t.Log(s)
}

type GetAppAccessToken struct {
	req.PostJson
	AppID        string `api:"body;$appid" json:"appId"`
	ClientSecret string `api:"body;$secret" json:"clientSecret"`
}

func (GetAppAccessToken) URL() string {
	return "https://bots.qq.com/app/getAppAccessToken"
}

func TestVariables(t *testing.T) {
	s, err := client.Text(GetAppAccessToken{})
	if err != nil {
		t.Fatal(err)
	}
	t.Log(s)
}
