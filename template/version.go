package template

import (
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
)

// Version 版本号
type Version struct {
	// 主版本号，当你做了不兼容的 API 修改时递增
	Major *uint64 `gorm:"index:idx_uses,priority:3"`

	// 次版本号，当你做了向下兼容的功能性新增时递增
	Minor *uint64 `gorm:"index:idx_uses,priority:4"`

	// 修订号，当你做了向下兼容的问题修正时递增
	Patch *uint64 `gorm:"index:idx_uses,priority:5"`
}

// String 打印版本信息
func (v Version) String() string {
	if v.Major == nil {
		return "<invalid Version>"
	} else if v.Minor == nil {
		return fmt.Sprintf("v%d", *v.Major)
	} else if v.Patch == nil {
		return fmt.Sprintf("v%d.%d", *v.Major, *v.Minor)
	}
	return fmt.Sprintf("v%d.%d.%d", *v.Major, *v.Minor, *v.Patch)
}

var _ fmt.Stringer = Version{}

// MarshalText 序列化文本
func (v Version) MarshalText() ([]byte, error) {
	return []byte(v.String()), nil
}

var _ encoding.TextMarshaler = Version{}

// ErrEmptyVersion 版本号不能为空
var ErrEmptyVersion = errors.New("req/template: Version cannot be empty")

// UnmarshalText 反序列化文本
func (v *Version) UnmarshalText(data []byte) (err error) {
	if len(data) == 0 {
		return ErrEmptyVersion
	}
	if data[0] != 'v' {
		return fmt.Errorf("req/template: invalid prefix %q", data[0])
	}
	// 解析整数指针的函数
	parsePart := func(part string) (*uint64, error) {
		clean := strings.TrimSpace(part)
		if clean == "" {
			return nil, ErrEmptyVersion
		}
		val, err := strconv.ParseUint(clean, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("req/template: invalid number %q: %w", clean, err)
		}
		return &val, nil
	}
	// 解析版本数字
	parts := strings.Split(string(data[1:]), ".")
	switch len(parts) {
	case 3:
		v.Patch, err = parsePart(parts[2])
		if err != nil {
			return
		}
		fallthrough
	case 2:
		v.Minor, err = parsePart(parts[1])
		if err != nil {
			return
		}
		fallthrough
	case 1:
		v.Major, err = parsePart(parts[0])
		return
	default:
		return fmt.Errorf("req/template: invalid format: expected vX.Y.Z, got: %q", string(data))
	}
}

var _ encoding.TextUnmarshaler = (*Version)(nil)

// MarshalJSON 序列化 JSON
func (v Version) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Quote(v.String())), nil
}

var _ json.Marshaler = Version{}

var versionType = reflect.TypeOf(Version{})

// UnmarshalJSON 反序列化 JSON
func (v *Version) UnmarshalJSON(b []byte) error {
	if len(b) <= 2 {
		return ErrEmptyVersion
	}
	// 去除 JSON 引号
	if b[0] != '"' || b[len(b)-1] != '"' {
		return &json.UnmarshalTypeError{
			Value: strconv.Quote(string(b)),
			Type:  versionType,
		}
	}
	return v.UnmarshalText(b[1 : len(b)-1])
}

var _ json.Unmarshaler = (*Version)(nil)

// MarshalYAML 序列化 YAML
func (v Version) MarshalYAML() (any, error) {
	return v.String(), nil
}

var _ yaml.Marshaler = Version{}

// UnmarshalYAML 反序列化 YAML
func (v *Version) UnmarshalYAML(value *yaml.Node) error {
	// 确保 YAML 节点是字符串类型
	if value.Tag != "!!str" {
		return &yaml.TypeError{
			Errors: []string{
				fmt.Sprintf("line %d: expected string scalar for Version, got %q", value.Line, value.Tag),
			},
		}
	}
	return v.UnmarshalText([]byte(value.Value))
}

var _ yaml.Unmarshaler = (*Version)(nil)

// Match 用于 GORM Scopes 动态筛选版本号
func (v *Version) Match(tx *gorm.DB) *gorm.DB {
	if v != nil {
		if v.Major != nil {
			tx = tx.Where("major = ?", *v.Major)
			if v.Minor != nil {
				tx = tx.Where("minor = ?", *v.Minor)
				if v.Patch != nil {
					tx = tx.Where("patch = ?", *v.Patch)
				}
			}
		}
	}
	return tx.Order("major DESC, minor DESC, patch DESC")
}
