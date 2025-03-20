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
	client = &req.Client{BaseURL: baseURL, Variables: map[string]any{
		"$appid":  "10086",
		"$secret": "...",
	}}
	client.SetAuthorization("abc123")
	client.SetUserAgent(req.UserAgent)
}

type minAPI struct {
	req.Get
	url string
}

func (m minAPI) RawURL() string {
	return m.url
}

func MinAPI(url string) minAPI {
	return minAPI{url: url}
}

func TestClient(t *testing.T) {
	// url starts with "/": use BaseURL
	m, err := client.JSON(MinAPI("/baseurl"))
	if err != nil {
		t.Fatal(m, err)
	}
	if m.(map[string]any)["url"] != "https://httpbin.org/anything/baseurl" {
		t.Fatal("request 1")
	}

	// url does not have a "/" prefix: BaseURL is not used
	m, err = client.JSON(MinAPI("https://httpbin.org/get?q=1"))
	if err != nil {
		t.Fatal(m, err)
	}
	if m.(map[string]any)["url"] != "https://httpbin.org/get?q=1" {
		t.Fatal("request 2")
	}
}

func TestCURL(t *testing.T) {
	s, err := client.CURL(MinAPI("https://httpbin.org/get?q=1"))
	if err != nil {
		t.Fatal(err)
	}
	t.Log(s)
}

type GetAppAccessToken struct {
	req.PostJSON
	AppID        string `api:"body:$appid" req:"appId"`
	ClientSecret string `api:"body:$secret" req:"clientSecret"`
	Empty1       string `api:"body"`
	Empty2       string `api:"body,omitempty"`
}

func (GetAppAccessToken) RawURL() string {
	return "https://httpbin.org/post"
}

func TestVariables(t *testing.T) {
	s, err := client.Text(GetAppAccessToken{})
	if err != nil {
		t.Fatal(err)
	}
	t.Log(s)
}
