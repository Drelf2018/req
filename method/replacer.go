package method

import (
	"strings"
	"unicode"
)

var (
	NameReplacer   = PascalToSnake
	HeaderReplacer = PascalToHyphenated
)

// PascalToSnake 将字符串中的大写字母替换为下划线加小写字母，大写首字母前不添加下划线，连续的大写字母只在第一个字母前添加下划线
func PascalToSnake(s string) string {
	if s == "" {
		return ""
	}
	var result strings.Builder
	for i, r := range s {
		if unicode.IsUpper(r) {
			// 不是第一个字符并且前一个不是大写字母，添加下划线
			if i > 0 && !unicode.IsUpper(rune(s[i-1])) {
				result.WriteRune('_')
			}
			// 转成小写字母
			result.WriteRune(unicode.ToLower(r))
		} else {
			// 非大写字母直接添加
			result.WriteRune(r)
		}
	}
	return result.String()
}

// PascalToHyphenated 将字符串中的大写字母替换为横杠加大写字母，大写首字母前不添加横杠，连续的大写字母只在第一个字母前添加横杠
func PascalToHyphenated(s string) string {
	if s == "" {
		return ""
	}
	var result strings.Builder
	for i, r := range s {
		// 本身是大写字母，不是第一个字符并且前一个不是大写字母，添加横杠
		if unicode.IsUpper(r) && i > 0 && !unicode.IsUpper(rune(s[i-1])) {
			result.WriteRune('-')
		}
		result.WriteRune(r)
	}
	return result.String()
}
