package req

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/format"
	"reflect"
	"regexp"
	"strings"
	"time"
)

var patternAllNumber = regexp.MustCompile(`^\d+$`)
var pattern2 = regexp.MustCompile(`(^|[^a-zA-Z])([a-z]+)`)
var pattern3 = regexp.MustCompile(`([A-Z])([a-z]+)`)
var pattern4 = regexp.MustCompile(`[^a-zA-Z0-9]`)

var numbers = map[byte]string{
	'0': "Zero_",
	'1': "One_",
	'2': "Two_",
	'3': "Three_",
	'4': "Four_",
	'5': "Five_",
	'6': "Six_",
	'7': "Seven_",
	'8': "Eight_",
	'9': "Nine_",
}

var Void = struct{}{}

var Abbreviation = map[string]struct{}{
	"API":   Void,
	"ASCII": Void,
	"CPU":   Void,
	"CSS":   Void,
	"DNS":   Void,
	"EOF":   Void,
	"GUID":  Void,
	"HTML":  Void,
	"HTTP":  Void,
	"HTTPS": Void,
	"ID":    Void,
	"IP":    Void,
	"JSON":  Void,
	"LHS":   Void,
	"QPS":   Void,
	"RAM":   Void,
	"RHS":   Void,
	"RPC":   Void,
	"SLA":   Void,
	"SMTP":  Void,
	"SSH":   Void,
	"TCP":   Void,
	"TLS":   Void,
	"TTL":   Void,
	"UDP":   Void,
	"UI":    Void,
	"UID":   Void,
	"UUID":  Void,
	"URI":   Void,
	"URL":   Void,
	"UTF8":  Void,
	"VM":    Void,
	"XML":   Void,
	"XSRF":  Void,
	"XSS":   Void,
}

func JavaScriptReplace(s string, pattern *regexp.Regexp, fn func(match string, submatch ...string) string) string {
	idx := pattern.FindAllStringSubmatchIndex(s, -1)
	if len(idx) == 0 {
		return s
	}
	buf := &bytes.Buffer{}
	for i, v := range pattern.FindAllStringSubmatch(s, -1) {
		if i == 0 {
			buf.WriteString(s[:idx[i][0]])
		} else {
			buf.WriteString(s[idx[i-1][1]:idx[i][0]])
		}
		buf.WriteString(fn(v[0], v[1:]...))
	}
	buf.WriteString(s[idx[len(idx)-1][1]:])
	return buf.String()
}

func RegularizeName(name string) string {
	name = JavaScriptReplace(name, pattern2, func(match string, submatch ...string) string {
		n := submatch[0]
		r := submatch[1]
		if _, ok := Abbreviation[strings.ToUpper(r)]; ok {
			return n + strings.ToUpper(r)
		} else {
			return n + strings.ToUpper(r[:1]) + strings.ToLower(r[1:])
		}
	})
	name = JavaScriptReplace(name, pattern3, func(match string, submatch ...string) string {
		n := submatch[0]
		r := submatch[1]
		if _, ok := Abbreviation[n+strings.ToUpper(r)]; ok {
			return strings.ToUpper(n + r)
		} else {
			return n + r
		}
	})
	return pattern4.ReplaceAllString(name, "")
}

func NormalizeName(name string) string {
	if name == "" {
		return ""
	}
	if patternAllNumber.MatchString(name) {
		name = "Num" + name
	} else if prefix, ok := numbers[name[0]]; ok {
		name = prefix + name[1:]
	}

	name = RegularizeName(name)
	if name != "" {
		return name
	}
	return "NamingFailed"
}

func ParseType(i any) string {
	switch i := i.(type) {
	case nil:
		return "any"
	case time.Time:
		return "time.Time"
	case []any:
		return "slice"
	case map[string]any:
		return "struct"
	case float64:
		if i-float64(int(i)) != 0 {
			return "float64"
		}
		if i > -2147483648 && i < 2147483647 {
			return "int"
		}
		return "int64"
	default:
		return reflect.TypeOf(i).Name()
	}
}

func ParseNumberType(type1, type2 string) string {
	switch {
	case strings.HasPrefix(type1, "float"):
		if strings.HasPrefix(type2, "int") {
			return type1
		} else if strings.HasPrefix(type2, "float") {
			return "float64"
		}
	case strings.HasPrefix(type1, "int"):
		if strings.HasPrefix(type2, "float") {
			return type2
		} else if strings.HasPrefix(type2, "int") {
			return "int64"
		}
	}
	return "any"
}

type Converter struct {
	buf *bytes.Buffer
	tab int
}

type item struct {
	val   any
	omit  bool
	count int
}

func (c *Converter) Add(s string) {
	c.buf.WriteString(s)
}

func (c *Converter) AddTab() {
	for i := 0; i < c.tab; i++ {
		c.buf.WriteByte('\t')
	}
}

func (c *Converter) AddStruct(items map[string]*item) {
	c.Add("struct {\n")
	c.tab++
	for key, item := range items {
		c.AddTab()
		c.Add(NormalizeName(key))
		c.Add(" ")
		comment := c.AddAny(item.val)
		c.Add(" `json:\"")
		c.Add(key)
		if item.omit {
			c.Add(",omitempty")
		}
		c.Add("\"`")
		if comment != "" {
			c.Add(" // ")
			c.Add(comment)
		}
		c.Add("\n")
	}
	c.tab--
	c.AddTab()
	c.Add("}")
}

func (c *Converter) AddAny(i any) (comment string) {
	switch i := i.(type) {
	case []any:
		length := len(i)
		var rtype string
		for idx := 0; idx < length; idx++ {
			if rtype == "" {
				rtype = ParseType(i[idx])
			} else if ntype := ParseType(i[idx]); rtype != ntype {
				rtype = ParseNumberType(ntype, rtype)
				if rtype == "any" {
					break
				}
			}
		}
		c.Add("[]")

		switch rtype {
		case "struct":
			items := make(map[string]*item)
			for idx := 0; idx < length; idx++ {
				iter := reflect.ValueOf(i[idx]).MapRange()
				for iter.Next() {
					key := iter.Key().String()
					if it, ok := items[key]; ok {
						it.count++
					} else {
						items[key] = &item{
							val:   iter.Value().Interface(),
							count: 1,
						}
					}
				}
			}
			for _, it := range items {
				it.omit = it.count != length
			}
			c.AddStruct(items)
			return ""
		case "slice":
			c.AddAny(i[0])
			return fmt.Sprint(i)
		case "":
			c.Add("any")
			return ""
		default:
			c.Add(rtype)
			return fmt.Sprint(i)
		}
	case map[string]any:
		items := make(map[string]*item)
		for key, val := range i {
			items[key] = &item{val: val}
		}
		c.AddStruct(items)
		return ""
	default:
		rtype := ParseType(i)
		c.Add(rtype)
		if rtype == "string" {
			s := i.(string)
			if s == "" {
				return "\"\""
			} else {
				return s
			}
		}
		return fmt.Sprint(i)
	}
}

func (c *Converter) JSONToStruct(b []byte, name string) ([]byte, error) {
	var i any
	err := json.Unmarshal(bytes.ReplaceAll(b, []byte(".0"), []byte(".1")), &i)
	if err != nil {
		return nil, err
	}
	name = NormalizeName(name)
	if name == "" {
		name = "AutoGenerated"
	}
	c.Add("type ")
	c.Add(name)
	c.Add(" ")
	c.AddAny(i)
	return format.Source(c.buf.Bytes())
}

func NewConverter() *Converter {
	return &Converter{
		buf: &bytes.Buffer{},
	}
}
