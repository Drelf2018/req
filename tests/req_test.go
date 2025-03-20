package req_test

import (
	"encoding/json"
	"io"
	"testing"

	"github.com/Drelf2018/req"
)

type User struct {
	req.Get
	UID string
}

func (u User) RawURL() string {
	return "https://httpbin.org/anything/user/" + u.UID
}

func TestUser(t *testing.T) {
	resp, err := req.Do(User{UID: "114514"})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(string(b))
}

type Sign struct {
	req.PostForm
	UID           int    `api:"query"`
	Sign          string `api:"body"`
	RefreshNow    bool   `api:"body"`
	Authorization string `api:"header"`
}

func (Sign) RawURL() string {
	return "https://httpbin.org/post"
}

func TestSign(t *testing.T) {
	i, err := req.JSON(Sign{
		UID:           114514,
		Sign:          "逸一时误一世",
		RefreshNow:    true,
		Authorization: "Token 1919810",
	})
	if err != nil {
		t.Fatal(err)
	}

	b, err := json.MarshalIndent(i, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(string(b))
}

// func TestGenerate(t *testing.T) {
// 	req.Generate("req_test.go", User{UID: "114514"})
// }

type Status struct {
	req.Get
}

func (Status) RawURL() string {
	return "https://httpbin.org/status/418"
}

func TestStatus(t *testing.T) {
	_, err := req.Text(Status{})
	t.Log(err)
}
