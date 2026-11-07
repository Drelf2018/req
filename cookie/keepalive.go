package cookie

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// State 是 http.CookieJar 的验证状态
type State int32

const (
	StateUnverified State = iota // 未验证
	StateVerifying               // 验证中
	StateVerified                // 已验证
	StateFailed                  // 验证失败
)

// Refresher 刷新器，用于刷新 http.CookieJar
type Refresher interface {
	Refresh(context.Context, http.CookieJar) error
}

// Validator 检测器，用于检测 http.CookieJar 是否有效，是刷新器的可选接口
type Validator interface {
	Valid(context.Context, http.CookieJar) (bool, error)
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

func (v *VerifyError) Unwrap() error { return v.Err }
func (v *VerifyError) Error() string { return fmt.Sprintf("req/cookie: %s: %s", v.Msg, v.Err) }

// Verify 检验 http.CookieJar 是否有效，如果已经失效会进行一次刷新
func Verify(ctx context.Context, refresher Refresher, jar http.CookieJar) (State, error) {
	// 实现了检测器则先判断是否有效
	if v, ok := refresher.(Validator); ok {
		valid, err := v.Valid(ctx, jar)
		if err != nil {
			return StateFailed, &VerifyError{Msg: "cannot validate jar", Err: err}
		}
		if valid {
			return StateVerified, nil
		}
	}
	// 没有实现检测器则直接刷新
	err := refresher.Refresh(ctx, jar)
	if err != nil {
		return StateFailed, &VerifyError{Msg: "cannot refresh jar", Err: err}
	}
	return StateVerified, nil
}

// Logger 日志，用于记录检验 http.CookieJar 时出现的错误
type Logger interface {
	ErrorContext(ctx context.Context, msg string, args ...any)
}

// KeepaliveCookieJar 是支持自动保活的 http.CookieJar
type KeepaliveCookieJar struct {
	// 任意 http.CookieJar 实例
	http.CookieJar

	// 刷新器
	Refresher Refresher

	// 日志
	Logger Logger

	// 当前状态
	state int32

	// 最近一次验证成功的时间
	verified time.Time

	// 保护 verified
	rw sync.RWMutex
}

// State 获取 http.CookieJar 的验证状态
func (k *KeepaliveCookieJar) State() State {
	return State(atomic.LoadInt32(&k.state))
}

// SinceVerified 距离上次验证成功的时间间隔
func (k *KeepaliveCookieJar) SinceVerified() time.Duration {
	k.rw.RLock()
	defer k.rw.RUnlock()
	return time.Since(k.verified)
}

// Verify 携带上下文检验 http.CookieJar ，无效时会尝试刷新一次，并更新验证状态。
// 出现错误时，如果设置了日志字段，会进行记录。
func (k *KeepaliveCookieJar) Verify(ctx context.Context) error {
	atomic.StoreInt32(&k.state, int32(StateVerifying))
	state, err := Verify(ctx, k.Refresher, k.CookieJar)
	atomic.StoreInt32(&k.state, int32(state))
	if err != nil {
		if k.Logger != nil {
			var v *VerifyError
			if errors.As(err, &v) {
				k.Logger.ErrorContext(ctx, v.Msg, "error", v.Err)
			} else {
				k.Logger.ErrorContext(ctx, "cannot verify jar", "error", err)
			}
		}
		return err
	}
	if state == StateVerified {
		k.rw.Lock()
		k.verified = time.Now()
		k.rw.Unlock()
	}
	return nil
}

// RunWithContext 效果同 Verify ，但不返回错误
func (k *KeepaliveCookieJar) RunWithContext(ctx context.Context) {
	_ = k.Verify(ctx)
}

// RunE 效果同 Verify ，但不需要传入上下文
func (k *KeepaliveCookieJar) RunE() error {
	return k.Verify(context.Background())
}

// Run 效果同 Verify ，但既不需要传入上下文也不返回错误
func (k *KeepaliveCookieJar) Run() {
	_ = k.RunE()
}
