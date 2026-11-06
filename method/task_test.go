package method

import (
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

const (
	srcEnv    = "REQ_TEST_SRC_ENV"
	srcCookie = "c"
	srcCtxKey = "k"
)

// cookieReader 造一个只用来读 cookie 的 *http.Request —— 它就是 CookieReader 的实现。
func cookieReader(kv ...string) CookieReader {
	h := make(http.Header)
	for i := 0; i+1 < len(kv); i += 2 {
		h.Add("Cookie", kv[i]+"="+kv[i+1])
	}
	return &http.Request{Header: h}
}

type tagFull struct {
	F string `req:"query:renamed" ctx:"k" cookie:"c" env:"REQ_TEST_SRC_ENV" default:"d"`
}

type tagOmit struct {
	F string `req:"query,omitempty"`
}

type tagRequired struct {
	F string `req:"query,required"`
}

type tagBothMods struct {
	F string `req:"query,omitempty,required"`
}

type tagDefaultOmit struct {
	F string `req:"query,omitempty" default:"d"`
}

type tagDefaultRequired struct {
	F string `req:"query,required" default:"d"`
}

type tagUnknownMod struct {
	F string `req:"query,nope"`
}

type tagNoTarget struct {
	F string `req:":name"`
}

type TagAnonInner struct {
	T string `req:"header"`
}

type tagAnonOuter struct {
	TagAnonInner `req:"body"`
}

type tagBodyIface struct {
	F any `req:"body" env:"REQ_TEST_SRC_ENV"`
}

type tagQueryIface struct {
	F any `req:"query" env:"REQ_TEST_SRC_ENV"`
}

// TestCompileTaskTag 覆盖“标签 → Field”这半边。
func TestCompileTaskTag(t *testing.T) {
	task, err := compileTask(reflect.TypeOf(tagFull{}))
	if err != nil {
		t.Fatal(err)
	}
	if len(task.Query) != 1 {
		t.Fatalf("want 1 query field, got %d", len(task.Query))
	}
	f := task.Query[0]
	if f.Name != "renamed" || f.Target != "query" {
		t.Errorf("名字/去向不对: %+v", f)
	}
	if f.CtxKey != "k" || f.CookieKey != "c" || f.EnvKey != srcEnv {
		t.Errorf("来源键不对: %+v", f)
	}
	if f.Default == nil || *f.Default != "d" {
		t.Errorf("默认值不对: %v", f.Default)
	}
	if len(f.Index) != 1 {
		t.Errorf("Index 不对: %v", f.Index)
	}
	for _, tt := range []struct {
		name    string
		v       any
		wantOmi bool
	}{
		{"omitempty", tagOmit{}, true},
		{"required", tagRequired{}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			task, err := compileTask(reflect.TypeOf(tt.v))
			if err != nil {
				t.Fatal(err)
			}
			if task.Query[0].Omit != tt.wantOmi || task.Query[0].Required == tt.wantOmi {
				t.Errorf("Omit=%v Required=%v", task.Query[0].Omit, task.Query[0].Required)
			}
		})
	}
}

// TestCompileTaskErrors 覆盖构建期能挡下来的各种标签错误。
//
// 注意“匿名字段带标签”那条：被嵌入的类型必须是【导出】的——不可导出的嵌入字段
// 会先被 IsExported 过滤掉，连匿名字段检查都走不到。
func TestCompileTaskErrors(t *testing.T) {
	for _, tt := range []struct {
		name string
		v    any
	}{
		{"omitempty 与 required 互斥", tagBothMods{}},
		{"default 与 omitempty 冲突", tagDefaultOmit{}},
		{"default 与 required 冲突", tagDefaultRequired{}},
		{"未知修饰符", tagUnknownMod{}},
		{"缺少去向", tagNoTarget{}},
		{"匿名字段带标签", tagAnonOuter{}},
		{"body + 接口 + 字符串来源", tagBodyIface{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := compileTask(reflect.TypeOf(tt.v)); err == nil {
				t.Error("应当报错")
			} else {
				t.Logf("→ %v", err)
			}
		})
	}
	// 回归保护：query 目标不需要解码器，接口类型照样能编码
	if _, err := compileTask(reflect.TypeOf(tagQueryIface{})); err != nil {
		t.Errorf("query + 接口 不该报错: %v", err)
	}
}

// srcQuery 七个字段分别命中链上的每一层，外加链尾两种策略。
type srcQuery struct {
	Field   string `req:"query" ctx:"k" cookie:"c" env:"REQ_TEST_SRC_ENV" default:"from-default"`
	Ctx     string `req:"query:ctx" ctx:"k"`
	Cookie  string `req:"query:cookie" cookie:"c"`
	Env     string `req:"query:env" env:"REQ_TEST_SRC_ENV"`
	Default string `req:"query:default" default:"from-default"`
	Omit    string `req:"query:omit,omitempty" env:"REQ_TEST_SRC_MISSING"`
	Req     string `req:"query:req,required" env:"REQ_TEST_SRC_MISSING"`
}

func srcValue() reflect.Value {
	return reflect.ValueOf(srcQuery{Field: "from-field"})
}

func srcContext() context.Context {
	return context.WithValue(context.Background(), srcCtxKey, "from-ctx")
}

func srcReader() CookieReader {
	return cookieReader(srcCookie, "from-cookie")
}

// TestToStringSources 钉住字符串目标：字段与链尾用编译期编码器，
// ctx 按动态类型归一化，cookie/env/default 原样一条。
func TestToStringSources(t *testing.T) {
	t.Setenv(srcEnv, "from-env")
	task, err := compileTask(reflect.TypeOf(srcQuery{}))
	if err != nil {
		t.Fatal(err)
	}
	v := srcValue()
	want := map[string][]string{
		"field":   {"from-field"},
		"ctx":     {"from-ctx"},
		"cookie":  {"from-cookie"},
		"env":     {"from-env"},
		"default": {"from-default"},
	}
	for i := range task.Query {
		f := task.Query[i]
		got, err := f.ToString(srcContext(), v, srcReader())
		switch f.Name {
		case "omit":
			if err != nil || got != nil {
				t.Errorf("omitempty 应当返回 nil, nil，得到 %#v %v", got, err)
			}
		case "req":
			if err != ErrMissing {
				t.Errorf("required 应当返回 ErrMissing，得到 %v", err)
			}
		default:
			if err != nil {
				t.Errorf("%s: %v", f.Name, err)
				continue
			}
			if !reflect.DeepEqual(got, want[f.Name]) {
				t.Errorf("%s: want %#v, got %#v", f.Name, want[f.Name], got)
			}
		}
	}
}

// TestToValueSources 钉住 json 目标：字段与 ctx 保留原类型，
// cookie/env/default 必须转成字段类型。
func TestToValueSources(t *testing.T) {
	t.Setenv(srcEnv, "from-env")
	task, err := compileTask(reflect.TypeOf(srcQuery{}))
	if err != nil {
		t.Fatal(err)
	}
	v := srcValue()
	want := map[string]string{
		"field":   "from-field",
		"ctx":     "from-ctx",
		"cookie":  "from-cookie",
		"env":     "from-env",
		"default": "from-default",
	}
	for i := range task.Query {
		f := task.Query[i]
		got, err := f.ToValue(srcContext(), v, srcReader())
		switch f.Name {
		case "omit":
			if err != nil || got != nil {
				t.Errorf("omitempty 应当返回 nil, nil，得到 %#v %v", got, err)
			}
		case "req":
			if err != ErrMissing {
				t.Errorf("required 应当返回 ErrMissing，得到 %v", err)
			}
		default:
			if err != nil {
				t.Errorf("%s: %v", f.Name, err)
				continue
			}
			if got != want[f.Name] {
				t.Errorf("%s: want %q, got %#v", f.Name, want[f.Name], got)
			}
		}
	}
}

// srcMiss 三个字段都声明了不存在的 cookie：命中失败必须【继续往下走】，
// 而不是把 http.ErrNoCookie 当成致命错误。
type srcMiss struct {
	WithDefault string `req:"query:withdefault" cookie:"nope" default:"d"`
	Required    string `req:"query:req,required" cookie:"nope"`
	Omit        string `req:"query:omit,omitempty" cookie:"nope"`
}

func TestCookieMissFallsThrough(t *testing.T) {
	task, err := compileTask(reflect.TypeOf(srcMiss{}))
	if err != nil {
		t.Fatal(err)
	}
	v := reflect.ValueOf(srcMiss{})
	for i := range task.Query {
		f := task.Query[i]
		got, err := f.ToString(context.Background(), v, cookieReader())
		switch f.Name {
		case "withdefault":
			if err != nil || !reflect.DeepEqual(got, []string{"d"}) {
				t.Errorf("未命中应当落到 default，得到 %#v %v", got, err)
			}
		case "req":
			if err != ErrMissing {
				t.Errorf("required 未命中应当 ErrMissing，得到 %v", err)
			}
		case "omit":
			if err != nil || got != nil {
				t.Errorf("omitempty 未命中应当 nil, nil，得到 %#v %v", got, err)
			}
		}
	}
}

type srcInt struct {
	F int `req:"body" default:"7"`
}

// TestStringVsValueTyped 两个目标的关键差异：default 在字符串目标上是原样字面量，
// 在 json 目标上要转成字段类型。
func TestStringVsValueTyped(t *testing.T) {
	task, err := compileTask(reflect.TypeOf(srcInt{}))
	if err != nil {
		t.Fatal(err)
	}
	f := task.Body[0]
	v := reflect.ValueOf(srcInt{})
	ss, err := f.ToString(context.Background(), v, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ss, []string{"7"}) {
		t.Errorf("ToString 应当给出字面量 7，得到 %#v", ss)
	}
	got, err := f.ToValue(context.Background(), v, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != 7 {
		t.Errorf("ToValue 应当给出 int(7)，得到 %#v (%T)", got, got)
	}
}

// testAPI 给测试用的结构体补上 API 的两个方法（MakeJSONMap 等现在收 API）。
type testAPI struct{}

func (testAPI) Method() string { return http.MethodGet }
func (testAPI) RawURL() string { return "/x" }

type mjBody struct {
	testAPI
	A int    `req:"body"`
	B string `req:"body" default:"bd"`
	C string `req:"body,omitempty" env:"REQ_TEST_SRC_MISSING"`
}

type mjRequired struct {
	testAPI
	D string `req:"body,required" env:"REQ_TEST_SRC_MISSING"`
}

// TestMakeJSONMap omitempty 未命中时应当【不出现这个键】，而不是写成 null；
// required 未命中时必须把 ErrMissing 传出来。
//
// 同时验证【结构体】与【指针】两种 API 形态——reflect.Indirect 少一次就会在指针上 panic。
func TestMakeJSONMap(t *testing.T) {
	task, err := compileTask(reflect.TypeOf(mjBody{}))
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name string
		api  API
	}{
		{"结构体", mjBody{A: 1}},
		{"指针", &mjBody{A: 1}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m, err := MakeMap(context.Background(), tt.api, nil, task.Body)
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := m["c"]; ok {
				t.Errorf("omitempty 未命中不该出现键，得到 %#v", m)
			}
			if m["a"] != 1 || m["b"] != "bd" {
				t.Errorf("want a=1 b=bd，得到 %#v", m)
			}
		})
	}
	req, err := compileTask(reflect.TypeOf(mjRequired{}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := MakeMap(context.Background(), mjRequired{}, nil, req.Body); err != ErrMissing {
		t.Errorf("required 未命中应当返回 ErrMissing，得到 %v", err)
	}
}

type srcSlice struct {
	testAPI
	IDs []int `req:"query:ids"`
}

// TestMakeURLValues 字符串目标会把切片逐元素展开成多条同名参数。
func TestMakeURLValues(t *testing.T) {
	task, err := compileTask(reflect.TypeOf(srcSlice{}))
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name string
		api  API
	}{
		{"结构体", srcSlice{IDs: []int{1, 2, 3}}},
		{"指针", &srcSlice{IDs: []int{1, 2, 3}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			u, err := MakeURLValues(context.Background(), tt.api, nil, task.Query)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(u["ids"], []string{"1", "2", "3"}) {
				t.Errorf("want ids=[1 2 3]，得到 %#v", u)
			}
			if u.Encode() != "ids=1&ids=2&ids=3" {
				t.Errorf("编码结果不对: %s", u.Encode())
			}
		})
	}
}

// TestMakeHeaderCanonical MakeHeader 必须规范化头名。
// 走 url.Values 的话 key 保持原样（"x-token"），Header.Get("X-Token") 会查不到。
func TestMakeHeaderCanonical(t *testing.T) {
	type headerAPI struct {
		testAPI
		Tag string `req:"header:x-token" default:"tok"`
	}
	task, err := compileTask(reflect.TypeOf(headerAPI{}))
	if err != nil {
		t.Fatal(err)
	}
	h, err := MakeHeader(context.Background(), headerAPI{}, nil, task.Header)
	if err != nil {
		t.Fatal(err)
	}
	if got := h.Get("X-Token"); got != "tok" {
		t.Errorf("Header.Get(X-Token) = %q，得到 %#v", got, h)
	}
}

// TestMakeCookiesOrder MakeCookies 按字段顺序产出，不经过 map（map 迭代顺序是随机的）。
func TestMakeCookiesOrder(t *testing.T) {
	type cookieAPI struct {
		testAPI
		A string `req:"cookie:a" default:"1"`
		B string `req:"cookie:b" default:"2"`
		C string `req:"cookie:c" default:"3"`
	}
	task, err := compileTask(reflect.TypeOf(cookieAPI{}))
	if err != nil {
		t.Fatal(err)
	}
	cs, err := MakeCookies(context.Background(), cookieAPI{}, nil, task.Cookie)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(cs))
	for _, c := range cs {
		names = append(names, c.Name)
	}
	if !reflect.DeepEqual(names, []string{"a", "b", "c"}) {
		t.Errorf("cookie 顺序 = %v，期望按字段顺序 a,b,c", names)
	}
}

// TestMakeQuery MakeQuery 只负责"做参数"，不碰请求；
// 与已有 query 的合并是 Session 的事（端到端见根包 TestExistingQueryPreserved）。
func TestMakeQuery(t *testing.T) {
	task, err := compileTask(reflect.TypeOf(srcSlice{}))
	if err != nil {
		t.Fatal(err)
	}
	q, err := MakeQuery(context.Background(), srcSlice{IDs: []int{1, 2}}, nil, task.Query)
	if err != nil {
		t.Fatal(err)
	}
	if q != "ids=1&ids=2" {
		t.Errorf("want ids=1&ids=2，得到 %q", q)
	}
	empty, err := MakeQuery(context.Background(), srcSlice{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if empty != "" {
		t.Errorf("空桶应当给出空串，得到 %q", empty)
	}
}

type mpAPI struct {
	PostMultipartForm
	Name string    `req:"body:name"`
	File io.Reader `req:"body:file"`
}

func (mpAPI) RawURL() string { return "/x" }

// TestPostMultipartForm 普通字段必须写进表单（之前 default 分支被注释掉，会被静默丢弃），
// io.Reader 字段当文件写，Content-Type 里带 boundary。
func TestPostMultipartForm(t *testing.T) {
	task, err := compileTask(reflect.TypeOf(mpAPI{}))
	if err != nil {
		t.Fatal(err)
	}
	api := mpAPI{Name: "hello", File: NewNamedReader("a.txt", []byte("content"))}
	b, hdr, err := api.Body(context.Background(), api, nil, task.Body)
	if err != nil {
		t.Fatal(err)
	}
	ctype := hdr.Get("Content-Type")
	_, params, err := mime.ParseMediaType(ctype)
	if err != nil {
		t.Fatalf("Content-Type 解析失败: %v", err)
	}
	form, err := multipart.NewReader(b, params["boundary"]).ReadForm(1 << 20)
	if err != nil {
		t.Fatal(err)
	}
	if got := form.Value["name"]; len(got) != 1 || got[0] != "hello" {
		t.Errorf("普通字段没写进表单: %#v", form.Value)
	}
	fh := form.File["file"]
	if len(fh) != 1 || fh[0].Filename != "a.txt" {
		t.Fatalf("文件字段不对: %#v", fh)
	}
	f, err := fh[0].Open()
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "content" {
		t.Errorf("文件内容不对: %q", data)
	}
	if !strings.HasPrefix(ctype, "multipart/form-data") {
		t.Errorf("Content-Type 不对: %q", ctype)
	}
}
