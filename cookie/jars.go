package cookie

import (
	"net/http"
	"net/url"
)

type Jars struct {
	cookies []*http.Cookie
	jars    []http.CookieJar
}

func (j *Jars) Add(key, value string) {
	j.cookies = append(j.cookies, &http.Cookie{Name: key, Value: value})
}

func (j *Jars) SetCookies(u *url.URL, cookies []*http.Cookie) {
	for _, jar := range j.jars {
		if jar != nil {
			jar.SetCookies(u, cookies)
		}
	}
}

func (j *Jars) Cookies(u *url.URL) []*http.Cookie {
	cookies := j.cookies
	for _, jar := range j.jars {
		if jar != nil {
			cookies = append(cookies, jar.Cookies(u)...)
		}
	}
	return cookies
}

var _ http.CookieJar = (*Jars)(nil)

func NewJars(jars ...any) *Jars {
	j := &Jars{}
	for _, jar := range jars {
		if jar, ok := jar.(http.CookieJar); ok && jar != nil {
			j.jars = append(j.jars, jar)
		}
	}
	return j
}
