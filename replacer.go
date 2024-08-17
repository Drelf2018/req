package req

import "strings"

var (
	keyReplacer    *strings.Replacer
	headerReplacer *strings.Replacer
)

func init() {
	oldnew1 := []string{"ID", "_id"}
	for i := 'A'; i <= 'Z'; i++ {
		oldnew1 = append(oldnew1, string(i)+"ID", "_"+string(i+32)+"id", string(i), "_"+string(i+32))
	}
	keyReplacer = strings.NewReplacer(oldnew1...)

	oldnew2 := make([]string, 0, 26*2)
	for i := 'A'; i <= 'Z'; i++ {
		oldnew2 = append(oldnew2, string(i), "-"+string(i))
	}
	headerReplacer = strings.NewReplacer(oldnew2...)
}

func KeyReplace(s string) string {
	return keyReplacer.Replace(s)[1:]
}

func HeaderReplace(s string) string {
	return headerReplacer.Replace(s)[1:]
}
