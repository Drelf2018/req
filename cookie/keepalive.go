package cookie

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"sync/atomic"
	"time"
)

// State 是 http.CookieJar 的验证状态
type State int32

const (
	StateUnverified State = iota // 未验证
	StateVerified                // 已验证
	StateFailed                  // 验证失败
)

// Refresher 刷新器，用于刷新 http.CookieJar
type Refresher interface {
	Refresh(context.Context, http.CookieJar) error
}

// Validator 检测器，是刷新器的可选接口，用于检测 http.CookieJar 是否有效
type Validator interface {
	Valid(context.Context, http.CookieJar) (bool, error)
}

// ValidateRefresher 是既能检测有效性、又能在失效时刷新的刷新器
type ValidateRefresher interface {
	Validator
	Refresher
}

// Namer 取名器，是 http.CookieJar 的可选接口，用于间接设置 go-cron 定时任务名
type Namer interface {
	Name() string
}

// RefreshFunc 刷新器函数
type RefreshFunc func(context.Context, http.CookieJar) error

func (f RefreshFunc) Refresh(ctx context.Context, jar http.CookieJar) error { return f(ctx, jar) }

var _ Refresher = (*RefreshFunc)(nil)

// VerifyError 检验 http.CookieJar 时出现的错误
type VerifyError struct {
	Msg string
	Err error
}

func (v *VerifyError) Unwrap() error {
	return v.Err
}

func (v *VerifyError) Error() string {
	return fmt.Sprintf("req/cookie: %s: %s", v.Msg, v.Err)
}

// Verify 检验 http.CookieJar 是否有效，如果已经失效会进行一次刷新
func Verify(ctx context.Context, refresher Refresher, jar http.CookieJar) error {
	// 实现了检测器则先判断是否有效
	if v, ok := refresher.(ValidateRefresher); ok {
		valid, err := v.Valid(ctx, jar)
		if err != nil {
			return &VerifyError{Msg: "cannot validate jar", Err: err}
		}
		if valid {
			return nil
		}
	}
	// 没有实现检测器则直接刷新
	err := refresher.Refresh(ctx, jar)
	if err != nil {
		return &VerifyError{Msg: "cannot refresh jar", Err: err}
	}
	return nil
}

// KeepaliveCookieJar 是支持自动保活的 http.CookieJar
type KeepaliveCookieJar struct {
	http.CookieJar // 任意 http.CookieJar 实例

	Refresher Refresher // 刷新器

	state    int32        // 当前 http.CookieJar 的验证状态
	verified atomic.Value // 最近一次验证成功的时间，类型为 time.Time
}

// State 获取 http.CookieJar 的验证状态
func (k *KeepaliveCookieJar) State() State {
	return State(atomic.LoadInt32(&k.state))
}

// SinceVerified 距离上次验证成功的时间间隔，从未验证过时返回 time.Duration 的最大值
func (k *KeepaliveCookieJar) SinceVerified() time.Duration {
	if v := k.verified.Load(); v != nil {
		return time.Since(v.(time.Time))
	}
	return math.MaxInt64
}

// Name 用于设置 go-cron 定时任务名
func (k *KeepaliveCookieJar) Name() string {
	if n, ok := k.CookieJar.(Namer); ok {
		if name := n.Name(); name != "" {
			return "keepalive_" + name
		}
	}
	return "keepalive"
}

var _ Namer = (*KeepaliveCookieJar)(nil)

// Verify 携带上下文检验 http.CookieJar，无效时会尝试刷新一次，并更新验证状态
func (k *KeepaliveCookieJar) Verify(ctx context.Context) error {
	err := Verify(ctx, k.Refresher, k.CookieJar)
	if err != nil {
		atomic.StoreInt32(&k.state, int32(StateFailed))
		return err
	}
	atomic.StoreInt32(&k.state, int32(StateVerified))
	k.verified.Store(time.Now())
	return nil
}

// RunE 效果同 Verify，但不需要传入上下文
func (k *KeepaliveCookieJar) RunE() error {
	return k.Verify(context.Background())
}

// Run 效果同 Verify，既不需要传入上下文也不返回错误，出错时 panic
func (k *KeepaliveCookieJar) Run() {
	if err := k.RunE(); err != nil {
		panic(err)
	}
}
