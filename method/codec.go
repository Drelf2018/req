package method

import (
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"sync"
)

// 本文件把字段类型在编译期编译成编解码器：每个字段确定一个 encodeFunc / decodeFunc，
// 运行期只做一次函数调用，不再反射判断类型。
//
// 核心约定：编码器的输出统一是 []string。
// 一个字段可能对应 0 到多条参数——切片要逐元素展开，所以“单值”只是“长度为 1 的切片”。
// 有了这个约定，容器类型（切片、数组、指针、接口）只需要递归自己，
// 再把子结果用 append 拼起来，嵌套多少层都会自动摊平。
//
// 解码是它的反向：输入是若干条字符串，标量要求恰好一条、切片逐元素合回；
// nil / 空 / 一条空串三档的含义由 boundary 统一。
//
// 两个方向的分类规则必须镜像（叶子 vs 容器），改动一边就要照看另一边。

var ErrNilType = errors.New("req/method: nil type")

// Encode 把任意值编码成它应该发出的全部字符串，容器类型会逐元素展开，叶子类型就是长度 1 的切片
func Encode(i any) ([]string, error) {
	if i == nil {
		return nil, nil
	}
	switch i := i.(type) {
	case bool:
		return []string{strconv.FormatBool(i)}, nil
	case int:
		return []string{strconv.FormatInt(int64(i), 10)}, nil
	case int8:
		return []string{strconv.FormatInt(int64(i), 10)}, nil
	case int16:
		return []string{strconv.FormatInt(int64(i), 10)}, nil
	case int32:
		return []string{strconv.FormatInt(int64(i), 10)}, nil
	case int64:
		return []string{strconv.FormatInt(i, 10)}, nil
	case uint:
		return []string{strconv.FormatUint(uint64(i), 10)}, nil
	case uint8:
		return []string{strconv.FormatUint(uint64(i), 10)}, nil
	case uint16:
		return []string{strconv.FormatUint(uint64(i), 10)}, nil
	case uint32:
		return []string{strconv.FormatUint(uint64(i), 10)}, nil
	case uint64:
		return []string{strconv.FormatUint(i, 10)}, nil
	case float32:
		return []string{strconv.FormatFloat(float64(i), 'f', -1, 32)}, nil
	case float64:
		return []string{strconv.FormatFloat(i, 'f', -1, 64)}, nil
	case complex64:
		return []string{strconv.FormatComplex(complex128(i), 'g', -1, 64)}, nil
	case complex128:
		return []string{strconv.FormatComplex(i, 'g', -1, 128)}, nil
	case []byte:
		return []string{string(i)}, nil
	case []string:
		return i, nil
	case string:
		return []string{i}, nil
	default:
		v := reflect.ValueOf(i)
		enc, err := loadEncode(v.Type())
		if err != nil {
			return nil, err
		}
		return enc(v)
	}
}

// DecodeType 把若干条字符串还原成 t 类型的值，是 Decode 的反射版。没有参数时返回 nil，传空切片则返回 t 的零值
func DecodeType(t reflect.Type, ss ...string) (any, error) {
	if t == nil {
		return nil, ErrNilType
	}
	d, err := loadDecode(t)
	if err != nil {
		return nil, err
	}
	return d(ss...)
}

// Decode 把若干条字符串还原成 T 类型的值，是 Encode 的反向。标量要求恰好一条，切片逐元素合回，数组要求条数等于长度
func Decode[T any](ss ...string) (T, error) {
	var zero T
	t := reflect.TypeOf((*T)(nil)).Elem()
	v, err := DecodeType(t, ss...)
	if err != nil {
		return zero, err
	}
	if v == nil {
		return zero, nil
	}
	got, ok := v.(T)
	if !ok {
		return zero, fmt.Errorf("req/method: want %v, got %T", t, v)
	}
	return got, nil
}

// encodeFunc 是编译后的编码器，把字段值转成它应该发出的全部字符串
type encodeFunc func(reflect.Value) ([]string, error)

// decodeFunc 是编译后的解码器，把若干条字符串转成具体类型值
type decodeFunc func(...string) (any, error)

// TypeCompileError 表示字段类型无法编译成编码器
type TypeCompileError struct {
	Type reflect.Type
}

func (e *TypeCompileError) Error() string {
	return fmt.Sprintf("req/method: cannot compile type %v", e.Type)
}

// 三个编码接口的类型，用来在编译期判断字段类型能否自行序列化
var (
	marshalerType     = reflect.TypeOf((*Marshaler)(nil)).Elem()
	jsonMarshalerType = reflect.TypeOf((*json.Marshaler)(nil)).Elem()
	textMarshalerType = reflect.TypeOf((*encoding.TextMarshaler)(nil)).Elem()
)

// 三个解码接口的类型，用来在编译期判断字段类型能否自行反序列化
var (
	unmarshalerType     = reflect.TypeOf((*Unmarshaler)(nil)).Elem()
	jsonUnmarshalerType = reflect.TypeOf((*json.Unmarshaler)(nil)).Elem()
	textUnmarshalerType = reflect.TypeOf((*encoding.TextUnmarshaler)(nil)).Elem()
)

// encodeCache map[reflect.Type]encodeFunc 缓存已编译的编码器
var encodeCache sync.Map

// decodeCache map[reflect.Type]decodeFunc 缓存已编译的解码器
var decodeCache sync.Map

// loadEncode 取出类型 t 的编码器，没编译过就编译并缓存
func loadEncode(t reflect.Type) (encodeFunc, error) {
	if v, ok := encodeCache.Load(t); ok {
		return v.(encodeFunc), nil
	}
	e, err := compileEncode(t)
	if err != nil {
		return nil, err
	}
	encodeCache.Store(t, e)
	return e, nil
}

// loadDecode 取出类型 t 的解码器，没编译过就编译并缓存
func loadDecode(t reflect.Type) (decodeFunc, error) {
	if v, ok := decodeCache.Load(t); ok {
		return v.(decodeFunc), nil
	}
	e, err := compileDecode(t)
	if err != nil {
		return nil, err
	}
	decodeCache.Store(t, e)
	return e, nil
}

// compileEncode 为类型 t 编译一个编码器。
//
// 分类规则是“叶子 vs 容器”：
//
//	叶子——实现了 Marshaler / json.Marshaler / encoding.TextMarshaler 的类型、
//	      各基础类型、struct 与 map（交给 encoding/json）、以及 []byte（当作整段字节串）。
//	容器——Slice / Array 逐元素摊平；Pointer 解引用透传；Interface 运行时按动态类型分派。
func compileEncode(t reflect.Type) (encodeFunc, error) {
	// 编码接口顺序：Marshaler → json.Marshaler → TextMarshaler
	switch {
	case t.Implements(marshalerType):
		return wrapErrEncode(marshalString), nil
	case t.Implements(jsonMarshalerType):
		return wrapErrEncode(marshalJSON), nil
	case t.Implements(textMarshalerType):
		return wrapErrEncode(marshalText), nil
	}

	// 按 Kind 分派。每个 Kind 单独一个 case，方便单独调整
	switch t.Kind() {
	case reflect.Bool:
		return wrapEncode(func(v reflect.Value) string {
			return strconv.FormatBool(v.Bool())
		}), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return wrapEncode(func(v reflect.Value) string {
			return strconv.FormatInt(v.Int(), 10)
		}), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return wrapEncode(func(v reflect.Value) string {
			return strconv.FormatUint(v.Uint(), 10)
		}), nil
	case reflect.Uintptr:
		return nil, &TypeCompileError{Type: t}
	case reflect.Float32:
		return wrapEncode(func(v reflect.Value) string {
			return strconv.FormatFloat(v.Float(), 'f', -1, 32)
		}), nil
	case reflect.Float64:
		return wrapEncode(func(v reflect.Value) string {
			return strconv.FormatFloat(v.Float(), 'f', -1, 64)
		}), nil
	case reflect.Complex64:
		return wrapEncode(func(v reflect.Value) string {
			return strconv.FormatComplex(v.Complex(), 'g', -1, 64)
		}), nil
	case reflect.Complex128:
		return wrapEncode(func(v reflect.Value) string {
			return strconv.FormatComplex(v.Complex(), 'g', -1, 128)
		}), nil
	case reflect.Array:
		return encodeSequence(t)
	case reflect.Chan:
		return nil, &TypeCompileError{Type: t}
	case reflect.Func:
		// 预留：之后可能会去实现“函数计算值再填入”的功能
		return nil, &TypeCompileError{Type: t}
	case reflect.Interface:
		return encodeInterface, nil
	case reflect.Map:
		return encodeJSON, nil
	case reflect.Pointer:
		return encodePointer(t)
	case reflect.Slice:
		// []byte（含具名字节切片）当作一整段字节串，不逐字节展开
		if t.Elem().Kind() == reflect.Uint8 {
			return wrapEncode(func(v reflect.Value) string {
				return string(v.Bytes())
			}), nil
		}
		return encodeSequence(t)
	case reflect.String:
		return wrapEncode(reflect.Value.String), nil
	case reflect.Struct:
		return encodeJSON, nil
	case reflect.UnsafePointer:
		return nil, &TypeCompileError{Type: t}
	default:
		return nil, &TypeCompileError{Type: t}
	}
}

// convertTo 把 v 转成 t，让具名类型也返回具名类型
func convertTo(t reflect.Type, v any) (any, error) {
	rv := reflect.ValueOf(v)
	if !rv.IsValid() || rv.Type() == t || !rv.Type().ConvertibleTo(t) {
		return v, nil
	}
	return rv.Convert(t).Interface(), nil
}

// boundary 处理“没值 / 空值”两个边界档：
//
//	ss == nil → nil（没值，调用方应当省略这个键）
//	len == 0  → t 的零值
//
// ok 为 true 表示调用方直接返回 v。reflect.Zero 只在“空值”这一罕见分支里执行，
// 所以不必预先缓存。
func boundary(t reflect.Type, ss []string) (v any, ok bool) {
	switch {
	case ss == nil:
		return nil, true
	case len(ss) == 0:
		return reflect.Zero(t).Interface(), true
	}
	return nil, false
}

// leafDecode 把“一个字符串 → 单值”的纯函数包成叶子解码器
func leafDecode[T any](t reflect.Type, fn func(string) (T, error)) decodeFunc {
	return func(ss ...string) (any, error) {
		if v, ok := boundary(t, ss); ok {
			return v, nil
		}
		if len(ss) != 1 {
			return nil, fmt.Errorf("req/method: %v needs exactly 1 string, got %d", t, len(ss))
		}
		v, err := fn(ss[0])
		if err != nil {
			return nil, fmt.Errorf("req/method: cannot parse %v: %w", t, err)
		}
		return convertTo(t, v)
	}
}

// compileDecode 为类型 t 编译一个解码器
func compileDecode(t reflect.Type) (decodeFunc, error) {
	// 编码接口顺序：Unmarshaler → json.Unmarshaler → TextUnmarshaler
	if k := t.Kind(); k != reflect.Pointer && k != reflect.Interface {
		ptr := reflect.PointerTo(t)
		switch {
		case ptr.Implements(unmarshalerType):
			return leafDecode(t, unmarshalString(t)), nil
		case ptr.Implements(jsonUnmarshalerType):
			return leafDecode(t, unmarshalJSON(t)), nil
		case ptr.Implements(textUnmarshalerType):
			return leafDecode(t, unmarshalText(t)), nil
		}
	}

	// 按 Kind 分派。每个 Kind 单独一个 case，方便单独调整
	switch kind := t.Kind(); kind {
	case reflect.Bool:
		return leafDecode(t, strconv.ParseBool), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return leafDecode(t, decodeInt(kind)), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return leafDecode(t, decodeUint(kind)), nil
	case reflect.Uintptr:
		return nil, &TypeCompileError{Type: t}
	case reflect.Float32, reflect.Float64:
		return leafDecode(t, decodeFloat(kind)), nil
	case reflect.Complex64, reflect.Complex128:
		return leafDecode(t, decodeComplex(kind)), nil
	case reflect.Array:
		return decodeArray(t)
	case reflect.Chan:
		return nil, &TypeCompileError{Type: t}
	case reflect.Func:
		return nil, &TypeCompileError{Type: t}
	case reflect.Interface:
		// 解码必须知道具体类型，而字符串不携带类型信息——空接口也不行，否则只能启发式
		// 猜（json 会把手里的 "5" 猜成 float64），往返一致性就没了。
		return nil, &TypeCompileError{Type: t}
	case reflect.Map:
		return leafDecode(t, decodeJSON(t)), nil
	case reflect.Pointer:
		return decodePointer(t)
	case reflect.Slice:
		if t.Elem().Kind() == reflect.Uint8 {
			return leafDecode(t, func(s string) ([]byte, error) { return []byte(s), nil }), nil
		}
		return decodeSlice(t)
	case reflect.String:
		return leafDecode(t, func(s string) (string, error) { return s, nil }), nil
	case reflect.Struct:
		return leafDecode(t, decodeJSON(t)), nil
	case reflect.UnsafePointer:
		return nil, &TypeCompileError{Type: t}
	default:
		return nil, &TypeCompileError{Type: t}
	}
}

// encodeSequence 为 Slice / Array 这类序列编译“逐元素摊平”的编码器
func encodeSequence(t reflect.Type) (encodeFunc, error) {
	elem, err := loadEncode(t.Elem())
	if err != nil {
		return nil, err
	}
	return func(v reflect.Value) ([]string, error) {
		if !v.IsValid() {
			return nil, nil
		}
		if v.Len() == 0 {
			if v.Kind() == reflect.Slice && v.IsNil() {
				return nil, nil
			}
			return []string{}, nil
		}
		out := make([]string, 0, v.Len())
		for i := 0; i < v.Len(); i++ {
			ss, err := elem(v.Index(i))
			if err != nil {
				return nil, err
			}
			out = append(out, ss...)
		}
		return out, nil
	}, nil
}

// decodeArray 为 [N]T 编译解码器，必须正好 N 条字符串，元素按各自的解码器逐个解析
func decodeArray(t reflect.Type) (decodeFunc, error) {
	elem, err := loadDecode(t.Elem())
	if err != nil {
		return nil, err
	}
	return func(ss ...string) (any, error) {
		if v, ok := boundary(t, ss); ok {
			return v, nil
		}
		// 不检查就是 Set 越界 → panic
		if len(ss) != t.Len() {
			return nil, fmt.Errorf("req/method: %v needs exactly %d strings, got %d", t, t.Len(), len(ss))
		}
		out := reflect.New(t).Elem() // 可寻址的 [N]T
		for i, s := range ss {
			v, err := elem(s)
			if err != nil {
				return nil, err
			}
			out.Index(i).Set(reflect.ValueOf(v))
		}
		return out.Interface(), nil
	}, nil
}

// decodeSlice 把字符串按元素类型逐个解码进切片
func decodeSlice(t reflect.Type) (decodeFunc, error) {
	elem, err := loadDecode(t.Elem())
	if err != nil {
		return nil, err
	}
	return func(ss ...string) (any, error) {
		if v, ok := boundary(t, ss); ok {
			return v, nil
		}
		out := reflect.MakeSlice(t, len(ss), len(ss))
		for i, s := range ss {
			v, err := elem(s) // 元素是叶子 → 正好 1 条；是序列 → 1 个元素
			if err != nil {
				return nil, err
			}
			if v == nil {
				continue // 元素没值 → 留零值
			}
			rv := reflect.ValueOf(v)
			if !rv.Type().AssignableTo(t.Elem()) {
				if !rv.Type().ConvertibleTo(t.Elem()) {
					return nil, fmt.Errorf("req/method: %v cannot assign %v", t, rv.Type())
				}
				rv = rv.Convert(t.Elem())
			}
			out.Index(i).Set(rv)
		}
		return out.Interface(), nil // 具名切片类型也保住
	}, nil
}

// encodePointer 为指针类型编译“解引用透传”的编码器
func encodePointer(t reflect.Type) (encodeFunc, error) {
	elem, err := loadEncode(t.Elem())
	if err != nil {
		return nil, err
	}
	return func(v reflect.Value) ([]string, error) {
		if !v.IsValid() || v.IsNil() {
			return nil, nil
		}
		return elem(v.Elem())
	}, nil
}

// decodePointer 为 *T 编译解码器，空值得到 nil 指针，其余交给元素解码器再取地址
func decodePointer(t reflect.Type) (decodeFunc, error) {
	elem, err := loadDecode(t.Elem())
	if err != nil {
		return nil, err
	}
	// boundary 里空值走 reflect.Zero(*T) → 正好是 nil，所以“空的指针来源”得到 nil，
	// 而不是 &T{}。N >= 1 交给元素，所以 *[]int 也能收 N 条。
	return func(ss ...string) (any, error) {
		if v, ok := boundary(t, ss); ok {
			return v, nil
		}
		v, err := elem(ss...)
		if err != nil || v == nil {
			return nil, err // 元素没值 → 保持 nil 指针
		}
		p := reflect.New(t.Elem())
		rv := reflect.ValueOf(v)
		if !rv.Type().AssignableTo(t.Elem()) {
			if !rv.Type().ConvertibleTo(t.Elem()) {
				return nil, fmt.Errorf("req/method: %v cannot assign %v", t, rv.Type())
			}
			rv = rv.Convert(t.Elem())
		}
		p.Elem().Set(rv)
		return convertTo(t, p.Interface())
	}, nil
}

// encodeInterface 处理接口类型的字段，具体类型只有运行时才知道
func encodeInterface(v reflect.Value) ([]string, error) {
	if !v.IsValid() || v.IsNil() {
		return nil, nil
	}
	v = v.Elem()
	e, err := loadEncode(v.Type())
	if err != nil {
		return nil, err
	}
	return e(v)
}

// wrapEncode 把“单值 → 一个字符串”的纯函数包成 encodeFunc
func wrapEncode(fn func(reflect.Value) string) encodeFunc {
	return func(v reflect.Value) ([]string, error) {
		if !v.IsValid() || isNilPointer(v) {
			return nil, nil
		}
		return []string{fn(v)}, nil
	}
}

// wrapErrEncode 与 wrapEncode 相同，但允许函数返回错误
func wrapErrEncode(fn func(reflect.Value) (string, error)) encodeFunc {
	return func(v reflect.Value) ([]string, error) {
		if !v.IsValid() || isNilPointer(v) {
			return nil, nil
		}
		s, err := fn(v)
		if err != nil {
			return nil, err
		}
		return []string{s}, nil
	}
}

// bitSizes 是各整数位宽对应的字节数的映射，用于把整数解码成字符串
var bitSizes = map[reflect.Kind]int{
	reflect.Int:        0,
	reflect.Int8:       8,
	reflect.Int16:      16,
	reflect.Int32:      32,
	reflect.Int64:      64,
	reflect.Uint:       0,
	reflect.Uint8:      8,
	reflect.Uint16:     16,
	reflect.Uint32:     32,
	reflect.Uint64:     64,
	reflect.Float32:    32,
	reflect.Float64:    64,
	reflect.Complex64:  64,
	reflect.Complex128: 128,
}

// decodeInt 按 kind 的位宽把字符串解析成对应的有符号整数
func decodeInt(kind reflect.Kind) func(s string) (any, error) {
	bitSize := bitSizes[kind]
	return func(s string) (any, error) {
		i, err := strconv.ParseInt(s, 10, bitSize)
		if err != nil {
			return nil, err
		}
		switch bitSize {
		case 0:
			return int(i), nil
		case 8:
			return int8(i), nil
		case 16:
			return int16(i), nil
		case 32:
			return int32(i), nil
		default:
			return i, nil
		}
	}
}

// decodeUint 按 kind 的位宽把字符串解析成对应的无符号整数
func decodeUint(kind reflect.Kind) func(s string) (any, error) {
	bitSize := bitSizes[kind]
	return func(s string) (any, error) {
		i, err := strconv.ParseUint(s, 10, bitSize)
		if err != nil {
			return nil, err
		}
		switch bitSize {
		case 0:
			return uint(i), nil
		case 8:
			return uint8(i), nil
		case 16:
			return uint16(i), nil
		case 32:
			return uint32(i), nil
		default:
			return i, nil
		}
	}
}

// decodeFloat 按 kind 的位宽把字符串解析成 float32 或 float64
func decodeFloat(kind reflect.Kind) func(s string) (any, error) {
	bitSize := bitSizes[kind]
	return func(s string) (any, error) {
		f, err := strconv.ParseFloat(s, bitSize)
		if err != nil {
			return nil, err
		}
		switch bitSize {
		case 32:
			return float32(f), nil
		default:
			return f, nil
		}
	}
}

// decodeComplex 按 kind 的位宽把字符串解析成 complex64 或 complex128
func decodeComplex(kind reflect.Kind) func(s string) (any, error) {
	bitSize := bitSizes[kind]
	return func(s string) (any, error) {
		c, err := strconv.ParseComplex(s, bitSize)
		if err != nil {
			return nil, err
		}
		switch bitSize {
		case 64:
			return complex64(c), nil
		default:
			return c, nil
		}
	}
}

// marshalString 是 Marshaler 编码接口对应的叶子函数
func marshalString(v reflect.Value) (string, error) {
	s, err := v.Interface().(Marshaler).MarshalString()
	if err != nil {
		return "", fmt.Errorf("req/method: cannot marshal %v: %w", v.Type(), err)
	}
	return s, nil
}

// marshalJSON 是 json.Marshaler 编码接口对应的叶子函数
func marshalJSON(v reflect.Value) (string, error) {
	b, err := v.Interface().(json.Marshaler).MarshalJSON()
	if err != nil {
		return "", fmt.Errorf("req/method: cannot marshal %v: %w", v.Type(), err)
	}
	return string(b), nil
}

// marshalText 是 encoding.TextMarshaler 编码接口对应的叶子函数
func marshalText(v reflect.Value) (string, error) {
	b, err := v.Interface().(encoding.TextMarshaler).MarshalText()
	if err != nil {
		return "", fmt.Errorf("req/method: cannot marshal %v: %w", v.Type(), err)
	}
	return string(b), nil
}

// unmarshalString 用字段类型自己实现的 Unmarshaler 把字符串解成值
func unmarshalString(t reflect.Type) func(string) (any, error) {
	return func(s string) (any, error) {
		p := reflect.New(t)
		if err := p.Interface().(Unmarshaler).UnmarshalString(s); err != nil {
			return nil, err
		}
		return p.Elem().Interface(), nil
	}
}

// unmarshalJSON 用字段类型自己实现的 json.UnmarshalJSON 把字符串解成值
func unmarshalJSON(t reflect.Type) func(string) (any, error) {
	return func(s string) (any, error) {
		p := reflect.New(t)
		if err := p.Interface().(json.Unmarshaler).UnmarshalJSON([]byte(s)); err != nil {
			return nil, err
		}
		return p.Elem().Interface(), nil
	}
}

// unmarshalText 用字段类型自己实现的 encoding.TextUnmarshaler 把字符串解成值
func unmarshalText(t reflect.Type) func(string) (any, error) {
	return func(s string) (any, error) {
		p := reflect.New(t)
		if err := p.Interface().(encoding.TextUnmarshaler).UnmarshalText([]byte(s)); err != nil {
			return nil, err
		}
		return p.Elem().Interface(), nil
	}
}

// encodeJSON 把 struct / map 整体交给 encoding/json 编码
func encodeJSON(v reflect.Value) ([]string, error) {
	if !v.IsValid() {
		return nil, nil
	}
	b, err := json.Marshal(v.Interface())
	if err != nil {
		return nil, fmt.Errorf("req/method: cannot marshal %v: %w", v.Type(), err)
	}
	return []string{string(b)}, nil
}

// decodeJSON 用 encoding/json 把字符串解成值
func decodeJSON(t reflect.Type) func(string) (any, error) {
	return func(s string) (any, error) {
		p := reflect.New(t)
		if err := json.Unmarshal([]byte(s), p.Interface()); err != nil {
			return nil, err // 交给 leafDecode 统一加前缀，别在这里包第二层
		}
		return p.Elem().Interface(), nil // ← 不是 p.Interface()
	}
}

// isNilPointer 判断 v 是不是 nil 指针，其它类型一律返回假
func isNilPointer(v reflect.Value) bool {
	return v.Kind() == reflect.Pointer && v.IsNil()
}
