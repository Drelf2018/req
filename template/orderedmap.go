package template

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/Drelf2018/req"
	"gopkg.in/yaml.v3"
)

// OrderedMap 有序映射
type OrderedMap struct {
	rw     sync.RWMutex
	keys   []string
	values map[string]any
}

// String 打印映射
func (o *OrderedMap) String() string {
	var b strings.Builder
	b.WriteByte('(')
	o.rw.RLock()
	for i, key := range o.keys {
		b.WriteString(key)
		b.WriteByte('=')
		value := o.values[key]
		if v, ok := value.(string); ok {
			b.WriteByte('"')
			b.WriteString(v)
			b.WriteByte('"')
		} else {
			b.WriteString(fmt.Sprint(value))
		}
		if i != len(o.keys)-1 {
			b.WriteString(", ")
		}
	}
	o.rw.RUnlock()
	b.WriteByte(')')
	return b.String()
}

// Len 获取键值对个数
func (o *OrderedMap) Len() (i int) {
	o.rw.RLock()
	i = len(o.keys)
	o.rw.RUnlock()
	return
}

// Get 获取值
func (o *OrderedMap) Get(key string) (value any, exists bool) {
	o.rw.RLock()
	if len(o.values) != 0 {
		value, exists = o.values[key]
	}
	o.rw.RUnlock()
	return
}

// set 设置键值对，它会检查键是否存在，存在则移动到末尾，不存在则追加到末尾，并更新值
//
//	这是一个无锁的私有方法，调用方必须自行加锁
func (o *OrderedMap) set(key string, value any) {
	// 更新值
	if o.values != nil {
		o.values[key] = value
	} else {
		o.values = map[string]any{key: value}
	}
	// 检查键是否已存在
	for i, k := range o.keys {
		if k == key {
			// 如果不是最后一个键，才需要移动到末尾
			if i != len(o.keys)-1 {
				// 将此位置之后的键前移一位，将目标键放在最后
				copy(o.keys[i:], o.keys[i+1:])
				o.keys[len(o.keys)-1] = key
			}
			return
		}
	}
	// 如果键不存在，则追加到末尾
	o.keys = append(o.keys, key)
}

// Set 设置键值对，它会检查键是否存在，存在则移动到末尾，不存在则追加到末尾，并更新值
func (o *OrderedMap) Set(key string, value any) {
	o.rw.Lock()
	o.set(key, value)
	o.rw.Unlock()
}

// Del 删除键值对
func (o *OrderedMap) Del(key string) {
	o.rw.Lock()
	if _, ok := o.values[key]; ok {
		delete(o.values, key)
		for i, k := range o.keys {
			if k == key {
				o.keys = append(o.keys[:i], o.keys[i+1:]...)
				break
			}
		}
	}
	o.rw.Unlock()
}

// Keys 获取所有键
func (o *OrderedMap) Keys() (keys []string) {
	o.rw.RLock()
	keys = append(keys, o.keys...)
	o.rw.RUnlock()
	return
}

// Values 获取所有值
func (o *OrderedMap) Values() (values []any) {
	o.rw.RLock()
	values = make([]any, 0, len(o.keys))
	for _, key := range o.keys {
		values = append(values, o.values[key])
	}
	o.rw.RUnlock()
	return
}

// Map 获取无序映射
func (o *OrderedMap) Map() (m map[string]any) {
	o.rw.RLock()
	m = make(map[string]any, len(o.keys))
	for k, v := range o.values {
		m[k] = v
	}
	o.rw.RUnlock()
	return
}

// Clone 复制映射
func (o *OrderedMap) Clone() *OrderedMap {
	o.rw.RLock()
	n := &OrderedMap{
		keys:   append([]string{}, o.keys...),
		values: make(map[string]any, len(o.keys)),
	}
	for k, v := range o.values {
		n.values[k] = v
	}
	o.rw.RUnlock()
	return n
}

// Update 更新映射，如果入参映射中的键已经存在，会将其移动到末尾，并且覆盖它的值
func (o *OrderedMap) Update(n *OrderedMap) *OrderedMap {
	if n == nil {
		return o
	}
	o.rw.Lock()
	n.rw.RLock()
	if o.values == nil {
		o.values = make(map[string]any, len(n.keys))
	}
	for k, v := range n.values {
		o.set(k, v)
	}
	n.rw.RUnlock()
	o.rw.Unlock()
	return o
}

// Iterate 按顺序遍历键值对，返回 false 时停止迭代
func (o *OrderedMap) Iterate(yield func(key string, value any) bool) {
	o.rw.RLock()
	defer o.rw.RUnlock()
	for _, key := range o.keys {
		if !yield(key, o.values[key]) {
			return
		}
	}
}

// Replace 替换值
func (o *OrderedMap) Replace(f func(key string, value any) (newValue any, shouldReplace bool)) {
	o.rw.Lock()
	for _, key := range o.keys {
		newValue, shouldReplace := f(key, o.values[key])
		if shouldReplace {
			o.values[key] = newValue
		}
	}
	o.rw.Unlock()
}

// UnmarshalJSON 反序列化 JSON
func (o *OrderedMap) UnmarshalJSON(b []byte) error {
	o.rw.Lock()
	defer o.rw.Unlock()
	o.keys = make([]string, 0)
	o.values = make(map[string]any)
	dec := json.NewDecoder(bytes.NewReader(b))
	// 验证第一个 Token 是否为对象的起始大括号
	token, err := dec.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return fmt.Errorf("req/template: invalid token type: expected json.Delim, got: %T", token)
	}
	if delim != '{' {
		return fmt.Errorf("req/template: invalid token: expected '{', got: %q", delim)
	}
	// 循环读取对象内的键值对，直到对象结束
	for dec.More() {
		// 确保键是字符串类型
		keyToken, err := dec.Token()
		if err != nil {
			return err
		}
		key, ok := keyToken.(string)
		if !ok {
			return fmt.Errorf("req/template: invalid token type: expected string, got: %T", keyToken)
		}
		// 解析值
		var raw json.RawMessage
		err = dec.Decode(&raw)
		if err != nil {
			return err
		}
		var value any
		if len(raw) != 0 && raw[0] == '{' {
			value = &OrderedMap{}
		}
		err = json.Unmarshal(raw, &value)
		if err != nil {
			return err
		}
		o.set(key, value)
	}
	// 验证最后一个 Token 是否为对象的结束大括号
	token, err = dec.Token()
	if err != nil {
		return err
	}
	delim, ok = token.(json.Delim)
	if !ok {
		return fmt.Errorf("req/template: invalid token type: expected json.Delim, got: %T", token)
	}
	if delim != '}' {
		return fmt.Errorf("req/template: invalid token: expected '}', got: %q", delim)
	}
	// 确保读取至末尾
	token, err = dec.Token()
	if err != io.EOF {
		return fmt.Errorf("req/template: invalid token: expected io.EOF, got: %v", token)
	}
	return nil
}

var _ json.Unmarshaler = (*OrderedMap)(nil)

// MarshalJSON 序列化 JSON
func (o *OrderedMap) MarshalJSON() ([]byte, error) {
	o.rw.RLock()
	defer o.rw.RUnlock()
	if o.values == nil {
		return []byte("null"), nil
	}
	if len(o.keys) == 0 {
		return []byte("{}"), nil
	}
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, key := range o.keys {
		// 序列化键
		keyJSON, err := json.Marshal(key)
		if err != nil {
			return nil, fmt.Errorf("%w: %q", err, key)
		}
		buf.Write(keyJSON)
		buf.WriteByte(':')
		// 序列化值
		valueJSON, err := json.Marshal(o.values[key])
		if err != nil {
			return nil, fmt.Errorf("%w: %q", err, key)
		}
		buf.Write(valueJSON)
		// 除最后一个键值对外，添加逗号分隔
		if i != len(o.keys)-1 {
			buf.WriteByte(',')
		}
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

var _ json.Marshaler = (*OrderedMap)(nil)

// UnmarshalYAML 反序列化 YAML
func (o *OrderedMap) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("req/template: invalid yaml node: expected mapping, got: %v", node.Kind)
	}
	o.rw.Lock()
	defer o.rw.Unlock()
	// YAML 映射节点的 Content 为键值对交替的切片
	o.keys = make([]string, 0, len(node.Content)/2)
	o.values = make(map[string]any, len(node.Content)/2)
	for i := 0; i < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		if keyNode.Kind != yaml.ScalarNode {
			return fmt.Errorf("req/template: invalid yaml key: expected scalar, got: %v", keyNode.Kind)
		}
		if i+1 >= len(node.Content) {
			return fmt.Errorf("req/template: invalid yaml index: %d", i+1)
		}
		// 解析值
		var value any
		valueNode := node.Content[i+1]
		if valueNode.Kind == yaml.MappingNode {
			value = &OrderedMap{}
		}
		if err := valueNode.Decode(&value); err != nil {
			return err
		}
		o.set(keyNode.Value, value)
	}
	return nil
}

var _ yaml.Unmarshaler = (*OrderedMap)(nil)

// MarshalYAML 序列化 YAML
func (o *OrderedMap) MarshalYAML() (any, error) {
	o.rw.RLock()
	defer o.rw.RUnlock()
	if o.values == nil {
		return nil, nil
	}
	node := &yaml.Node{Kind: yaml.MappingNode}
	node.Content = make([]*yaml.Node, 0, len(o.keys)*2)
	for _, key := range o.keys {
		// 序列化键
		keyNode := &yaml.Node{}
		if err := keyNode.Encode(key); err != nil {
			return nil, err
		}
		// 序列化值
		valueNode := &yaml.Node{}
		if err := valueNode.Encode(o.values[key]); err != nil {
			return nil, err
		}
		node.Content = append(node.Content, keyNode, valueNode)
	}
	return node, nil
}

var _ yaml.Marshaler = (*OrderedMap)(nil)

// Getter 按顺序从有序映射中查找键值
type Getter []*OrderedMap

var ErrKeyNotFound = errors.New("key not found")

// Get 返回第一个找到键的有序映射中的值，如果都没有会返回错误
func (g Getter) Get(key string) (any, error) {
	for _, m := range g {
		v, ok := m.Get(key)
		if ok {
			return v, nil
		}
	}
	return nil, fmt.Errorf("%w: %q", ErrKeyNotFound, key)
}

// TemplateError 模板错误
type TemplateError OrderedMap

// Len 获取错误个数
func (t *TemplateError) Len() int {
	return (*OrderedMap)(t).Len()
}

// Add 添加错误
func (t *TemplateError) Add(key string, err error) *TemplateError {
	(*OrderedMap)(t).Set(key, err)
	return t
}

// Error 获取错误信息
func (t *TemplateError) Error() string {
	count := t.Len()
	if count == 0 {
		return ""
	}
	var b strings.Builder
	(*OrderedMap)(t).Iterate(func(key string, value any) bool {
		var s string
		if err, ok := value.(error); ok {
			s = err.Error()
		} else {
			s = fmt.Sprint(value)
		}
		b.WriteString(key)
		for _, line := range strings.Split(s, "\n") {
			b.WriteString("\n  ")
			b.WriteString(line)
		}
		count--
		if count != 0 {
			b.WriteByte('\n')
		}
		return true
	})
	return b.String()
}

var _ error = (*TemplateError)(nil)

// Unwrap 获取解包错误
func (t *TemplateError) Unwrap() error {
	if t.Len() != 0 {
		return t
	}
	return nil
}

var _ req.Unwrap = (*TemplateError)(nil)

// orderedMapToTemplateError 将有序映射转换成模板错误
func orderedMapToTemplateError(o *OrderedMap) *TemplateError {
	if o == nil {
		return nil
	}
	o.rw.Lock()
	for _, key := range o.keys {
		if v, ok := o.values[key].(*OrderedMap); ok {
			if _, ok := v.Get("error"); !ok {
				o.values[key] = orderedMapToTemplateError(v)
			}
		}
	}
	o.rw.Unlock()
	return (*TemplateError)(o)
}

// UnmarshalJSON 反序列化 JSON
func (t *TemplateError) UnmarshalJSON(b []byte) error {
	err := (*OrderedMap)(t).UnmarshalJSON(b)
	orderedMapToTemplateError((*OrderedMap)(t))
	return err
}

var _ json.Unmarshaler = (*TemplateError)(nil)

// WrapError 将错误包装成包含更多信息的结构体
func (t *TemplateError) WrapError() {
	t.rw.Lock()
	for _, key := range t.keys {
		switch v := t.values[key].(type) {
		case *TemplateError:
			v.WrapError()
		case error:
			t.values[key] = struct {
				Type  string `json:"type"`
				Error string `json:"error"`
				Value error  `json:"value"`
			}{fmt.Sprintf("%T", v), v.Error(), v}
		}
	}
	t.rw.Unlock()
}

// MarshalJSON 序列化 JSON
func (t *TemplateError) MarshalJSON() ([]byte, error) {
	t.WrapError()
	return (*OrderedMap)(t).MarshalJSON()
}

var _ json.Marshaler = (*TemplateError)(nil)

// UnmarshalYAML 反序列化 YAML
func (t *TemplateError) UnmarshalYAML(node *yaml.Node) error {
	err := (*OrderedMap)(t).UnmarshalYAML(node)
	orderedMapToTemplateError((*OrderedMap)(t))
	return err
}

var _ yaml.Unmarshaler = (*TemplateError)(nil)

// MarshalYAML 序列化 YAML
func (t *TemplateError) MarshalYAML() (any, error) {
	t.WrapError()
	return (*OrderedMap)(t).MarshalYAML()
}

var _ yaml.Marshaler = (*TemplateError)(nil)
