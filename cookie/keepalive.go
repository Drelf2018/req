package cookie

import (
	"context"
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
	Refresh(ctx context.Context, jar http.CookieJar) error
}

// Validator 检测器，用于检测 http.CookieJar 是否有效
type Validator interface {
	Valid(ctx context.Context, jar http.CookieJar) (bool, error)
}

// RefreshFunc 刷新器函数
type RefreshFunc func(context.Context, http.CookieJar) error

func (f RefreshFunc) Refresh(ctx context.Context, jar http.CookieJar) error { return f(ctx, jar) }

var _ Refresher = (*RefreshFunc)(nil)

// Verify 检验 http.CookieJar 是否有效，如果已经失效会进行一次刷新
func Verify(ctx context.Context, refresher Refresher, jar http.CookieJar) (State, error) {
	// 实现了检测器则先判断是否有效
	if v, ok := refresher.(Validator); ok {
		valid, err := v.Valid(ctx, jar)
		if err != nil {
			return StateFailed, fmt.Errorf("req/cookie: cannot validate jar: %w", err)
		}
		if valid {
			return StateVerified, nil
		}
	}
	// 没有实现检测器则直接刷新
	err := refresher.Refresh(ctx, jar)
	if err != nil {
		return StateFailed, fmt.Errorf("req/cookie: cannot refresh jar: %w", err)
	}
	return StateVerified, nil
}

// KeepaliveCookieJar 是支持自动保活的 http.CookieJar
type KeepaliveCookieJar struct {
	// 任意 http.CookieJar 实例
	http.CookieJar

	// 刷新器
	Refresher Refresher

	// 错误上报
	OnError func(error)

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

// Verify 携带上下文检验 http.CookieJar ，无效时会尝试刷新一次，并更新验证状态
func (k *KeepaliveCookieJar) Verify(ctx context.Context) error {
	atomic.StoreInt32(&k.state, int32(StateVerifying))
	state, err := Verify(ctx, k.Refresher, k.CookieJar)
	atomic.StoreInt32(&k.state, int32(state))
	if err != nil {
		return err
	}
	if state == StateVerified {
		k.rw.Lock()
		k.verified = time.Now()
		k.rw.Unlock()
	}
	return nil
}

// Run 检验 http.CookieJar ，错误交给 OnError ，适合 cron 这类要求无返回值的场景
func (k *KeepaliveCookieJar) Run() {
	err := k.Verify(context.Background())
	if err != nil && k.OnError != nil {
		k.OnError(err)
	}
}
