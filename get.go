package req

import "net/http"

type GetURL[T any] string

func (api GetURL[T]) URL() string {
	return string(api)
}

func (GetURL[T]) Method() string {
	return http.MethodGet
}

func (api GetURL[T]) Do() (T, error) {
	return Do[T](api)
}
