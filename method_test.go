package req_test

import (
	"io"
	"testing"

	"github.com/Drelf2018/req"
)

var cli = req.MustClone("https://api.bilibili.com/x/web-interface")

type Card struct {
	req.Get
	MID int `api:"query"`
}

func (Card) RawURL() string {
	return "/card"
}

var _ req.API = Card{}

func TestCard(t *testing.T) {
	s, err := cli.Text(Card{MID: 114514})
	if err != nil {
		t.Fatal(err)
	}
	t.Log(s)
}

type b6 struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

type PostJSON struct {
	req.PostJSON
	B1  string   `api:"body"`
	B2  int      `api:"body"`
	B3  float64  `api:"body"`
	B4  bool     `api:"body"`
	B5  []string `api:"body"`
	B6  b6       `api:"body"`
	MID int      `api:"query"`
}

func (PostJSON) RawURL() string {
	return "https://httpbin.org/post"
}

var _ req.API = PostJSON{}

func TestPostJSON(t *testing.T) {
	data, err := cli.JSON(PostJSON{
		B1: "https://api.bilibili.com/x/web-interface/card",
		B5: []string{"114", "514"},
		B6: b6{
			Name: "nana7mi",
			Age:  17,
		},
		MID: 114514,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Log(data)
}

type file struct {
	name  string
	value string
}

func (f file) Name() string {
	return f.name
}

func (f file) Read(p []byte) (n int, err error) {
	return copy(p, []byte(f.value)), io.EOF
}

var _ req.NamedReader = file{}

type Upload struct {
	req.PostMultipartForm
	FileA  file   `api:"file"`
	FileB  file   `api:"file" req:"file_c"`
	FilesC []file `api:"files"`
	FilesD []file `api:"files" req:"upload/files_e"`
}

func (Upload) RawURL() string {
	return "https://httpbin.org/post"
}

var _ req.API = Upload{}

type MyFileWriter struct {
	req.DefaultFileWriter
}

func (w MyFileWriter) Write(file io.Reader, data req.Field) error {
	switch file := file.(type) {
	case req.NamedReader:
		return req.WriteFormFile(w.Writer, data.Name, file.Name(), file)
	default:
		return req.WriteFormFile(w.Writer, data.Name, data.Value, file)
	}
}

var _ req.FileWriter = &MyFileWriter{}

func TestPostMultipartForm(t *testing.T) {
	var data = Upload{
		// PostMultipartForm: req.PostMultipartForm{&MyFileWriter{}},

		FileA:  file{"a.txt", "hello A!"},
		FileB:  file{"b.txt", "hello B!"},
		FilesC: []file{{"c1.txt", "hello C1!"}, {"c2.txt", "hello C2!"}},
		FilesD: []file{{"d1.txt", "hello D1!"}, {"d2.txt", "hello D2!"}},
	}
	r, err := cli.JSON(data)
	if err != nil {
		t.Fatal(err)
	}
	r2 := r.(map[string]any)
	for key, value := range r2["files"].(map[string]any) {
		t.Logf("%v: %v", key, value)
	}
}

// func TestGenerate(t *testing.T) {
// 	err := cli.Generate("method_test.go", PostJSON{MID: 114514})
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	r, err := GetCard(cli, Card{MID: 114514})
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	t.Log(r)
// }
