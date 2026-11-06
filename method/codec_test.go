package method

import (
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"
)

type codecAge int
type codecStr string

var fixedTime = time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)

// TestEncodeTiers 钉住 []string 的三档含义：
//
//	nil         没有值（无效值、nil 指针、nil 切片、Encode(nil)）
//	[]string{}  值存在但是空集合（非 nil 空切片、[0]T 数组）
//	[]string{""} 值存在且是一条空串（"" 字段、空 []byte、Marshaler 返回 ""）
func TestEncodeTiers(t *testing.T) {
	tests := []struct {
		name    string
		in      any
		want    []string
		wantNil bool
	}{
		{"nil", nil, nil, true},
		{"nil 切片", []int(nil), nil, true},
		{"nil string 切片", []string(nil), nil, true},
		{"nil 指针", (*int)(nil), nil, true},
		{"非 nil 空切片 → 空集合", []int{}, []string{}, false},
		{"非 nil 空 string 切片", []string{}, []string{}, false},
		{"零长数组 → 空集合", [0]int{}, []string{}, false},
		{"空字符串 → 一条空串", "", []string{""}, false},
		{"空 []byte → 一条空串", []byte{}, []string{""}, false},
		{"nil []byte → 一条空串", []byte(nil), []string{""}, false},
		{"普通值", 42, []string{"42"}, false},
		{"切片展开", []int{1, 2}, []string{"1", "2"}, false},
		{"[]byte 是一整段", []byte("ab"), []string{"ab"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Encode(tt.in)
			if err != nil {
				t.Fatalf("意外错误: %v", err)
			}
			if tt.wantNil && got != nil {
				t.Errorf("want nil, got %#v", got)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("want %#v, got %#v", tt.want, got)
			}
		})
	}
}

// TestEncodeFastPathMatchesCompiled 保证 Encode 里的 type switch 快路径
// 与编译期编码器给出完全一样的结果——快路径是复制出来的，会漂移。
func TestEncodeFastPathMatchesCompiled(t *testing.T) {
	values := []any{
		true, int(-5), int8(-3), int16(1), int32(2), int64(3),
		uint(5), uint8(6), uint16(7), uint32(8), uint64(9),
		float32(1.5), float64(2.5), complex64(1 + 2i), complex128(3 + 4i),
		"", "abc", []byte(nil), []byte("xy"), []byte{},
	}
	for _, v := range values {
		enc, err := loadEncode(reflect.TypeOf(v))
		if err != nil {
			t.Errorf("%T: loadEncode 报错 %v", v, err)
			continue
		}
		fast, errFast := Encode(v)
		slow, errSlow := enc(reflect.ValueOf(v))
		if (errFast == nil) != (errSlow == nil) {
			t.Errorf("%T: 快路径 err=%v，编译路径 err=%v", v, errFast, errSlow)
			continue
		}
		if !reflect.DeepEqual(fast, slow) {
			t.Errorf("%T: 快路径 %#v，编译路径 %#v", v, fast, slow)
		}
	}
}

// TestEncodeDecodeRoundTrip 是两边的主契约：Encode 展开成多条，Decode 就合回。
func TestEncodeDecodeRoundTrip(t *testing.T) {
	values := []any{
		int(-5), int8(-3), uint16(7), true, "", "abc", 3.5, float32(1.5),
		[]int{1, 2}, []string{"a", "b"}, []byte("xy"), [2]int{3, 4},
		codecAge(18), codecStr("hi"), []codecAge{1, 2}, fixedTime,
	}
	for _, v := range values {
		ss, err := Encode(v)
		if err != nil {
			t.Errorf("%T: Encode 报错 %v", v, err)
			continue
		}
		got, err := DecodeType(reflect.TypeOf(v), ss...)
		if err != nil {
			t.Errorf("%T: Decode 报错 %v（Encode 给了 %q）", v, err, ss)
			continue
		}
		if !reflect.DeepEqual(got, v) {
			t.Errorf("%T: want %#v, got %#v (%T)", v, v, got, got)
		}
	}
}

// TestDecodeTiers 钉住元数规则：
//
//	ss == nil → nil（没值，调用方应当省略这个键）
//	len == 0  → t 的零值
//	标量恰好一条；切片逐元素合回；数组要求条数等于长度
func TestDecodeTiers(t *testing.T) {
	tests := []struct {
		name    string
		typ     reflect.Type
		ss      []string
		want    any
		wantErr bool
	}{
		{"标量/没值", reflect.TypeOf(0), nil, nil, false},
		{"标量/空值 → 零值", reflect.TypeOf(0), []string{}, 0, false},
		{"标量/一条", reflect.TypeOf(0), []string{"18"}, 18, false},
		{"标量/两条 → 报错", reflect.TypeOf(0), []string{"1", "2"}, nil, true},

		{"字符串/空值 → 空串", reflect.TypeOf(""), []string{}, "", false},
		{"具名类型保真", reflect.TypeOf(codecAge(0)), []string{"18"}, codecAge(18), false},
		{"具名字符串保真", reflect.TypeOf(codecStr("")), []string{"x"}, codecStr("x"), false},

		{"切片/两条", reflect.TypeOf([]int(nil)), []string{"1", "2"}, []int{1, 2}, false},
		{"切片/空值 → nil", reflect.TypeOf([]int(nil)), []string{}, []int(nil), false},
		{"数组/三条", reflect.TypeOf([3]int{}), []string{"1", "2", "3"}, [3]int{1, 2, 3}, false},
		{"数组/两条 → 报错", reflect.TypeOf([3]int{}), []string{"1", "2"}, nil, true},
		{"数组/没值 → nil", reflect.TypeOf([3]int{}), nil, nil, false},

		{"指针/一条", reflect.TypeOf((*int)(nil)), []string{"18"}, ptrInt(18), false},
		{"指针/空值 → nil", reflect.TypeOf((*int)(nil)), []string{}, (*int)(nil), false},
		{"指针切片/两条", reflect.TypeOf((*[]int)(nil)), []string{"1", "2"}, ptrSlice([]int{1, 2}), false},

		{"接口 → 编译期拒绝", reflect.TypeOf((*any)(nil)).Elem(), []string{"x"}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DecodeType(tt.typ, tt.ss...)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("期望报错，得到 %#v", got)
				}
				t.Logf("→ %v", err)
				return
			}
			if err != nil {
				t.Fatalf("意外错误: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("want %#v (%T), got %#v (%T)", tt.want, tt.want, got, got)
			}
		})
	}
}

// TestDecodeErrors 覆盖各形态的解析失败与非法输入。
func TestDecodeErrors(t *testing.T) {
	tests := []struct {
		name string
		typ  reflect.Type
		s    string
	}{
		{"非法数字", reflect.TypeOf(0), "abc"},
		{"int8 越界", reflect.TypeOf(int8(0)), "200"},
		{"uint8 越界", reflect.TypeOf(uint8(0)), "300"},
		{"float32 越界", reflect.TypeOf(float32(0)), "1e40"},
		{"complex64 越界", reflect.TypeOf(complex64(0)), "1e40+0i"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, err := DecodeType(tt.typ); err != nil || got != nil {
				t.Errorf("没值应当给 nil: %#v %v", got, err)
			}
			if _, err := DecodeType(tt.typ, tt.s); err == nil {
				t.Error("应当报错")
			} else {
				t.Logf("→ %v", err)
			}
		})
	}
}

// TestDecodeNilType 覆盖反射版内核的入参校验。
func TestDecodeNilType(t *testing.T) {
	if _, err := DecodeType(nil, "x"); err == nil {
		t.Error("nil 类型应当报错")
	} else {
		t.Logf("→ %v", err)
	}
}

// TestDecodeGeneric 覆盖泛型外壳：拿到的是 T，而不是 any。
func TestDecodeGeneric(t *testing.T) {
	if v, err := Decode[int]("18"); err != nil || v != 18 {
		t.Errorf("int: %#v %v", v, err)
	}
	if v, err := Decode[codecAge]("18"); err != nil || v != codecAge(18) {
		t.Errorf("具名类型: %#v %v", v, err)
	}
	if v, err := Decode[[]int]("1", "2"); err != nil || !reflect.DeepEqual(v, []int{1, 2}) {
		t.Errorf("切片: %#v %v", v, err)
	}
	const jsonTime = `"2024-01-01T00:00:00Z"`
	if v, err := Decode[time.Time](jsonTime); err != nil || v.Year() != 2024 {
		t.Errorf("time.Time: %#v %v", v, err)
	}
	// 已知损失：泛型外壳拿不到“没值”这个信号，没参数与空切片都退化成零值
	if v, err := Decode[int](); err != nil || v != 0 {
		t.Errorf("没参数: %#v %v", v, err)
	}
	if v, err := Decode[*int](); err != nil || v != nil {
		t.Errorf("没参数的指针: %#v %v", v, err)
	}
}

// TestEncodeTypeCompileError 编码侧无法编译的类型应当报 TypeCompileError。
func TestEncodeTypeCompileError(t *testing.T) {
	for _, v := range []any{make(chan int), func() {}} {
		if _, err := Encode(v); err == nil {
			t.Errorf("%T: 应当报错", v)
		} else {
			t.Logf("%T → %v", v, err)
		}
	}
}

func ptrInt(i int) *int       { return &i }
func ptrSlice(s []int) *[]int { return &s }

type failMarshaler struct{}

func (failMarshaler) MarshalString() (string, error) { return "", errors.New("boom") }

type failUnmarshaler struct{}

func (*failUnmarshaler) UnmarshalString(string) error { return errors.New("boom") }

type badJSON struct{ C chan int }

// TestStdlibErrorPrefix 钉住两件事：
//
//  1. stdlib 传上来的错误也要带 req/method: 前缀；
//  2. 前缀是一句把上下文（类型 / 字段）融进去的话，stdlib 的原话用 %w 原样跟在冒号后面。
func TestStdlibErrorPrefix(t *testing.T) {
	cases := []struct {
		name string
		run  func() error
		want string
	}{
		{"strconv 解析", func() error { _, err := DecodeType(reflect.TypeOf(0), "abc"); return err },
			"req/method: cannot parse int: strconv.ParseInt:"},
		{"json 编码", func() error { _, err := Encode(badJSON{}); return err },
			"req/method: cannot marshal method.badJSON: json:"},
		{"json 解码", func() error { _, err := DecodeType(reflect.TypeOf(struct{ A int }{}), "x"); return err },
			"req/method: cannot parse struct { A int }: invalid character"},
		{"json 解码（切片）", func() error { _, err := DecodeType(reflect.TypeOf([]struct{ A int }{}), "x"); return err },
			"req/method: cannot parse"},
		{"Marshaler 实现报错", func() error { _, err := Encode(failMarshaler{}); return err },
			"req/method: cannot marshal method.failMarshaler: boom"},
		{"Unmarshaler 实现报错", func() error { _, err := DecodeType(reflect.TypeOf(failUnmarshaler{}), "x"); return err },
			"req/method: cannot parse method.failUnmarshaler: boom"},
		{"NewJSONReader", func() error { _, err := NewJSONReader(map[string]any{"x": math.NaN()}); return err },
			"req/method: cannot marshal map[string]interface {}: json:"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.run()
			if err == nil {
				t.Fatal("应当报错")
			}
			if !strings.HasPrefix(err.Error(), "req/method: ") {
				t.Errorf("缺前缀: %v", err)
			}
			// 嵌套包装很容易包出第二个前缀，这里钉死只有一个
			if n := strings.Count(err.Error(), "req/method:"); n != 1 {
				t.Errorf("req/method: 出现了 %d 次: %v", n, err)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("want 含 %q，得到 %v", c.want, err)
			}
			t.Logf("→ %v", err)
		})
	}
}
