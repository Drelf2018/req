package method

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
)

// Marshal 将任意类型转换为字符串
func Marshal(i any) string {
	if i == nil {
		return ""
	}
	switch i := i.(type) {
	case Marshaler:
		return i.MarshalString()
	case []byte:
		return string(i)
	case string:
		return i
	case bool:
		if i {
			return "true"
		}
		return "false"
	case int:
		return strconv.FormatInt(int64(i), 10)
	case int8:
		return strconv.FormatInt(int64(i), 10)
	case int16:
		return strconv.FormatInt(int64(i), 10)
	case int32:
		return strconv.FormatInt(int64(i), 10)
	case int64:
		return strconv.FormatInt(i, 10)
	case uint:
		return strconv.FormatUint(uint64(i), 10)
	case uint8:
		return strconv.FormatUint(uint64(i), 10)
	case uint16:
		return strconv.FormatUint(uint64(i), 10)
	case uint32:
		return strconv.FormatUint(uint64(i), 10)
	case uint64:
		return strconv.FormatUint(i, 10)
	case float32:
		return strconv.FormatFloat(float64(i), 'f', -1, 32)
	case float64:
		return strconv.FormatFloat(float64(i), 'f', -1, 64)
	default:
		b, _ := json.Marshal(i)
		return string(b)
	}
}

// Unmarshal 将字符串转换为指定类型的值
func Unmarshal(value reflect.Value, s string) (any, error) {
	switch value.Kind() {
	case reflect.Bool:
		return strconv.ParseBool(s)
	case reflect.Int:
		return strconv.ParseInt(s, 10, 0)
	case reflect.Int8:
		return strconv.ParseInt(s, 10, 8)
	case reflect.Int16:
		return strconv.ParseInt(s, 10, 16)
	case reflect.Int32:
		return strconv.ParseInt(s, 10, 32)
	case reflect.Int64:
		return strconv.ParseInt(s, 10, 64)
	case reflect.Uint:
		return strconv.ParseUint(s, 10, 0)
	case reflect.Uint8:
		return strconv.ParseUint(s, 10, 8)
	case reflect.Uint16:
		return strconv.ParseUint(s, 10, 16)
	case reflect.Uint32:
		return strconv.ParseUint(s, 10, 32)
	case reflect.Uint64:
		return strconv.ParseUint(s, 10, 64)
	case reflect.Float32:
		return strconv.ParseFloat(s, 32)
	case reflect.Float64:
		return strconv.ParseFloat(s, 64)
	case reflect.Pointer:
		if value.IsNil() {
			value.Set(reflect.New(value.Type().Elem()))
		}
		return Unmarshal(value.Elem(), s)
	case reflect.String:
		return s, nil
	default:
		return nil, fmt.Errorf("req/method.Unmarshal: unsupported type: %s", value.Kind())
	}
}

// AddValue 根据提供的 StructField 添加对应的值
func AddValue(ctx context.Context, val reflect.Value, field reflect.StructField, add func(string, string)) {
	// 一般不会有人传入与字段索引不对应的值，这里做个保险
	val, err := val.FieldByIndexErr(field.Index)
	if err != nil {
		return
	}
	// 当字段值为空时，先通过标签是否被清空判断是否忽略这项，如果不忽略则判断是否设置了默认值
	// 如果默认值以 "$" 开头则从上下文中获取变量值并添加，否则直接添加默认值
	// 如果未设置默认值，则流转到下面根据字段类型添加零值的逻辑
	if val.IsZero() {
		if len(field.Tag) == 0 {
			return
		} else if v, ok := field.Tag.Lookup("default"); ok {
			if strings.HasPrefix(v, "$") {
				i := ctx.Value(v)
				if i != nil {
					add(field.Name, Marshal(i))
				}
			} else {
				add(field.Name, v)
			}
			return
		}
	}
	// 如果是数组或切片类型，则逐个添加元素值，否则根据字段的类型添加值
	switch val.Kind() {
	case reflect.Array, reflect.Slice:
		for i := 0; i < val.Len(); i++ {
			add(field.Name, Marshal(val.Index(i).Interface()))
		}
	default:
		add(field.Name, Marshal(val.Interface()))
	}
}

// MakeJSONMap 根据提供的 []StructField 制作 map[string]any
func MakeJSONMap(ctx context.Context, val reflect.Value, body []reflect.StructField) map[string]any {
	m := make(map[string]any, len(body))
	for _, field := range body {
		// 一般不会有人传入与字段索引不对应的值，这里做个保险
		item, err := val.FieldByIndexErr(field.Index)
		if err != nil {
			continue
		}
		// 当字段值为空时，先通过标签是否被清空判断是否忽略这项，如果不忽略则判断是否设置了默认值
		// 如果默认值以 "$" 开头则从上下文中获取变量值并添加，否则直接添加默认值
		// 如果未设置默认值，则流转到下面根据字段类型添加零值的逻辑
		if item.IsZero() {
			if len(field.Tag) == 0 {
				continue
			} else if v, ok := field.Tag.Lookup("default"); ok {
				var i any
				if strings.HasPrefix(v, "$") {
					i = ctx.Value(v)
				} else {
					i, _ = Unmarshal(item, v)
				}
				if i != nil {
					m[field.Name] = i
				}
				continue
			}
		}
		// 直接添加值
		m[field.Name] = item.Interface()
	}
	return m
}

// MakeURLValues 根据提供的 []StructField 制作 url.Values
func MakeURLValues(ctx context.Context, val reflect.Value, fields []reflect.StructField) url.Values {
	u := make(url.Values, len(fields))
	for _, field := range fields {
		AddValue(ctx, val, field, u.Add)
	}
	return u
}

// AddCookie 向 req 请求添加 Cookie
func AddCookie(req *http.Request, val reflect.Value, cookie []reflect.StructField) {
	ctx := req.Context()
	for _, field := range cookie {
		AddValue(ctx, val, field, func(name, value string) { req.AddCookie(&http.Cookie{Name: name, Value: value}) })
	}
}

// AddQuery 向 req 请求添加请求参数
func AddQuery(req *http.Request, val reflect.Value, query []reflect.StructField) {
	q := MakeURLValues(req.Context(), val, query)
	if len(q) != 0 {
		req.URL.RawQuery = q.Encode()
	}
}

// AddHeader 向 req 请求添加请求头
func AddHeader(req *http.Request, val reflect.Value, header []reflect.StructField) {
	ctx := req.Context()
	for _, field := range header {
		AddValue(ctx, val, field, req.Header.Add)
	}
}
