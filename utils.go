package req

import (
	"context"
	"net/http"
)

func NewRequestWithContext(ctx context.Context, api Api) (req *http.Request, err error) {
	return DefaultClient.NewRequestWithContext(ctx, api)
}

func NewRequest(api Api) (req *http.Request, err error) {
	return DefaultClient.NewRequest(api)
}

func DoWithContext[T any](ctx context.Context, api Api) (zero T, err error) {
	err = DefaultClient.DoWithContext(ctx, api, &zero)
	return
}

func Do[T any](api Api) (T, error) {
	return DoWithContext[T](context.Background(), api)
}

func CURL(api Api) (string, error) {
	return DefaultClient.CURL(api)
}

func Debug(api Api) (map[string]any, error) {
	return DefaultClient.Debug(api)
}
