package req

import "strings"

var replacer *strings.Replacer

func init() {
	oldnew := []string{"ID", "_id"}
	for i := 'A'; i <= 'Z'; i++ {
		oldnew = append(oldnew, string(i), "_"+string(i+32))
	}
	replacer = strings.NewReplacer(oldnew...)
}

func Replace(s string) string {
	return replacer.Replace(s)[1:]
}
