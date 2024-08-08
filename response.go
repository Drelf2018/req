package req

import (
	"context"
)

func DoWithContext[T any](ctx context.Context, api Api) (zero T, err error) {
	err = DefaultClient.DoWithContext(ctx, api, &zero)
	return
}

func Do[T any](api Api) (T, error) {
	return DoWithContext[T](context.Background(), api)
}

func Debug(api Api) (map[string]any, error) {
	return Do[map[string]any](api)
}
