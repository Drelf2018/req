package cookie

import (
	"net/http"
	"testing"
)

type testCred struct {
	Token   string `cookie:"token"`
	Session string `cookie:"session_id"`
}

var testCookies = []*http.Cookie{
	{Name: "token", Value: "x"},
	{Name: "session_id", Value: "y"},
}

// TestGetSetRoundTrip 指针的读写往返
func TestGetSetRoundTrip(t *testing.T) {
	c := &testCred{}
	if err := Set(c, testCookies); err != nil {
		t.Fatal(err)
	}
	if c.Token != "x" || c.Session != "y" {
		t.Fatalf("写回失败: %+v", c)
	}
	got, err := Get(c)
	if err != nil {
		t.Fatal(err)
	}
	m := map[string]string{}
	for _, x := range got {
		m[x.Name] = x.Value
	}
	if m["token"] != "x" || m["session_id"] != "y" || len(m) != 2 {
		t.Fatalf("读取失败: %+v", m)
	}
	// 顺序按字段声明，必须稳定
	if got[0].Name != "token" || got[1].Name != "session_id" {
		t.Errorf("顺序不稳定: %v, %v", got[0].Name, got[1].Name)
	}
}

// TestSetRequiresPointer 传结构体值必须报错，不能静默失效。
//
// 之前的实现会照常返回 nil，写入落在临时拷贝上，调用方看不到任何变化。
func TestSetRequiresPointer(t *testing.T) {
	c := testCred{}
	if err := Set(c, testCookies); err == nil {
		t.Fatal("传值应当报错")
	}
	if c.Token != "" || c.Session != "" {
		t.Fatalf("不该被修改: %+v", c)
	}
}

// TestGetRequiresPointer 传值也要报错（行为变更：以前能读，现在统一要求指针）
func TestGetRequiresPointer(t *testing.T) {
	c := testCred{Token: "x"}
	if _, err := Get(c); err == nil {
		t.Fatal("传值应当报错")
	}
}

// TestNilPointer 非 nil 接口里的 nil 指针
func TestNilPointer(t *testing.T) {
	var c *testCred
	if err := Set(c, testCookies); err == nil {
		t.Fatal("nil 指针应当报错")
	}
	if _, err := Get(c); err == nil {
		t.Fatal("nil 指针应当报错")
	}
}

// TestNonStruct 非结构体
func TestNonStruct(t *testing.T) {
	n := 42
	if _, err := Get(&n); err == nil {
		t.Fatal("非结构体应当报错")
	}
}

type InnerCred struct {
	Token string `cookie:"token"`
}

type outerCred struct {
	InnerCred
	Session string `cookie:"session_id"`
}

// TestEmbeddedStruct 嵌入结构体里的 cookie 字段也要能找到（BFS 队列）
func TestEmbeddedStruct(t *testing.T) {
	c := &outerCred{InnerCred: InnerCred{Token: "t"}, Session: "s"}
	got, err := Get(c)
	if err != nil {
		t.Fatal(err)
	}
	m := map[string]string{}
	for _, x := range got {
		m[x.Name] = x.Value
	}
	if m["token"] != "t" || m["session_id"] != "s" || len(m) != 2 {
		t.Fatalf("嵌入结构体字段丢失: %+v", m)
	}
	// 写回也要落到嵌入字段上
	if err := Set(c, []*http.Cookie{{Name: "token", Value: "t2"}}); err != nil {
		t.Fatal(err)
	}
	if c.Token != "t2" {
		t.Fatalf("写回没落到嵌入字段: %+v", c)
	}
}

type withRest struct {
	Token string            `cookie:"token"`
	Rest  map[string]string `cookie:",embed"`
}

// TestEmbedGet 兜底槽里的键值要出现在结果里，且不能崩
func TestEmbedGet(t *testing.T) {
	c := &withRest{Token: "t", Rest: map[string]string{"extra": "e", "b_nut": "n"}}
	got, err := Get(c)
	if err != nil {
		t.Fatal(err)
	}
	m := map[string]string{}
	for _, x := range got {
		if _, dup := m[x.Name]; dup {
			t.Errorf("重复的名字 %q", x.Name)
		}
		m[x.Name] = x.Value
	}
	if m["token"] != "t" || m["extra"] != "e" || m["b_nut"] != "n" || len(m) != 3 {
		t.Fatalf("兜底槽没生效: %+v", m)
	}
}

// TestEmbedSetUnknown 没声明的 cookie 要进兜底槽，nil map 要自动初始化
func TestEmbedSetUnknown(t *testing.T) {
	c := &withRest{}
	if err := Set(c, []*http.Cookie{
		{Name: "token", Value: "t"},
		{Name: "b_nut", Value: "n"},
	}); err != nil {
		t.Fatal(err)
	}
	if c.Token != "t" {
		t.Errorf("声明字段没写进去: %+v", c)
	}
	if c.Rest["b_nut"] != "n" {
		t.Errorf("未知 cookie 没进兜底槽: %+v", c.Rest)
	}
}

// TestEmbedDeclaredWins 两边都有同名时，以声明字段为准且只输出一次
func TestEmbedDeclaredWins(t *testing.T) {
	c := &withRest{Token: "declared", Rest: map[string]string{"token": "fromMap"}}
	got, err := Get(c)
	if err != nil {
		t.Fatal(err)
	}
	var n int
	for _, x := range got {
		if x.Name == "token" {
			n++
			if x.Value != "declared" {
				t.Errorf("声明字段应当优先: %q", x.Value)
			}
		}
	}
	if n != 1 {
		t.Errorf("token 出现了 %d 次", n)
	}
}

// TestNoEmbedDropsUnknown 没有兜底槽时，未知 cookie 仍然被丢弃（现状，记录行为）
func TestNoEmbedDropsUnknown(t *testing.T) {
	c := &testCred{}
	if err := Set(c, []*http.Cookie{{Name: "b_nut", Value: "n"}}); err != nil {
		t.Fatal(err)
	}
	if c.Token != "" || c.Session != "" {
		t.Errorf("不该被写入: %+v", c)
	}
}

type badEmbed struct {
	Token string         `cookie:"token"`
	Rest  map[string]int `cookie:",embed"`
}

// TestEmbedWrongType embed 字段类型不对必须报错，不能静默忽略
func TestEmbedWrongType(t *testing.T) {
	c := &badEmbed{}
	if _, err := Get(c); err == nil {
		t.Fatal("map[string]int 的 embed 应当报错")
	} else {
		t.Logf("Get 报错: %v", err)
	}
	if err := Set(c, []*http.Cookie{{Name: "a", Value: "1"}}); err == nil {
		t.Fatal("Set 也应当报错")
	}
}

// embedFirst 兜底槽声明在同名字段【之前】
type embedFirst struct {
	Rest  map[string]string `cookie:",embed"`
	Token string            `cookie:"token"`
}

// TestEmbedDeclaredWinsReversed 声明字段在后时，也必须只输出一次且以声明为准
func TestEmbedDeclaredWinsReversed(t *testing.T) {
	c := &embedFirst{Rest: map[string]string{"token": "fromMap"}, Token: "declared"}
	got, err := Get(c)
	if err != nil {
		t.Fatal(err)
	}
	var n int
	for _, x := range got {
		if x.Name == "token" {
			n++
			if x.Value != "declared" {
				t.Errorf("声明字段应当优先: %q", x.Value)
			}
		}
	}
	if n != 1 {
		t.Errorf("token 出现了 %d 次（顺序依赖 bug）: %v", n, names(got))
	}
}

func names(cs []*http.Cookie) []string {
	out := make([]string, 0, len(cs))
	for _, c := range cs {
		out = append(out, c.Name+"="+c.Value)
	}
	return out
}
