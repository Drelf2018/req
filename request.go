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
