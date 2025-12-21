package cookie

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"
)

// State 是 Cookie 的验证状态
type State int32

const (
	StateUnverified State = iota // 未验证
	StateVerifying               // 验证中
	StateVerified                // 已验证
	StateInvalid                 // 已失效
)

// RefreshableCookieJar 是支持检测 Cookie 是否有效和刷新的 http.CookieJar
type RefreshableCookieJar interface {
	http.CookieJar

	// IsValid 用于检测 Cookie 是否有效
	IsValid(context.Context) (bool, error)

	// Refresh 用于刷新 Cookie
	Refresh(context.Context) error
}

// Verify 检测 Cookie 是否有效，如果已经失效会进行一次刷新
func Verify(ctx context.Context, jar RefreshableCookieJar) (State, error) {
	valid, err := jar.IsValid(ctx)
	if err != nil {
		return StateUnverified, fmt.Errorf("req/cookie.Verify: verify cookie failed: %w", err)
	}
	if valid {
		return StateVerified, nil
	}
	err = jar.Refresh(ctx)
	if err != nil {
		return StateInvalid, fmt.Errorf("req/cookie.Verify: refresh cookie failed: %w", err)
	}
	return StateVerified, nil
}

// KeepaliveCookieJar 是支持自动保活的 CookieJar
type KeepaliveCookieJar struct {
	// 可刷新的 CookieJar 实例
	RefreshableCookieJar

	// 保活过程中出现错误时自动调用
	OnError func(k *KeepaliveCookieJar, err error)

	// 最小验证间隔，当字段值不为零时，本次验证时间距离上次成功验证时间超过该值时，才会执行验证，当字段值为零时，每次都会执行
	MinVerifyInterval time.Duration

	// 上次验证成功时间
	lastVerifiedTime time.Time

	// 主动取消保活
	cancel context.CancelFunc

	// 当前状态
	state int32
}

// State 获取 Cookie 的验证状态
func (k *KeepaliveCookieJar) State() State {
	return State(atomic.LoadInt32(&k.state))
}

// Verify 立即检测 Cookie 是否有效
func (k *KeepaliveCookieJar) Verify(ctx context.Context) {
	if k.MinVerifyInterval != 0 && time.Since(k.lastVerifiedTime) <= k.MinVerifyInterval {
		return
	}
	atomic.StoreInt32(&k.state, int32(StateVerifying))
	state, err := Verify(ctx, k.RefreshableCookieJar)
	atomic.StoreInt32(&k.state, int32(state))
	if err != nil {
		if k.OnError != nil {
			k.OnError(k, err)
		} else if v, ok := k.RefreshableCookieJar.(interface{ OnError(error) }); ok {
			v.OnError(err)
		}
	} else {
		if state == StateVerified {
			k.lastVerifiedTime = time.Now()
		}
	}
}

// Cookies 设置了最小验证间隔时，每次获取 Cookie 前会进行检测
func (k *KeepaliveCookieJar) Cookies(u *url.URL) []*http.Cookie {
	if k.MinVerifyInterval != 0 {
		k.Verify(context.Background())
	}
	return k.RefreshableCookieJar.Cookies(u)
}

// Keepalive 自动保活 Cookie
func (k *KeepaliveCookieJar) Keepalive(ctx context.Context, refresh time.Duration, now bool) {
	// 可以主动取消
	ctx, k.cancel = context.WithCancel(ctx)
	defer k.cancel()
	// 立即进行一次检测
	if now {
		k.Verify(ctx)
	}
	// 每间隔固定时间进行一次检测
	ticker := time.NewTicker(refresh)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			if ctx.Err() != nil {
				if k.OnError != nil {
					k.OnError(k, ctx.Err())
				} else if v, ok := k.RefreshableCookieJar.(interface{ OnError(error) }); ok {
					v.OnError(ctx.Err())
				}
			}
			return
		case <-ticker.C:
			k.Verify(ctx)
		}
	}
}

// StopKeepalive 主动取消保活 Cookie
func (k *KeepaliveCookieJar) StopKeepalive() {
	if k.cancel != nil {
		k.cancel()
	}
}

// KeepaliveWithContext 立即开始保活 RefreshableCookieJar
func KeepaliveWithContext(ctx context.Context, refresh time.Duration, jar RefreshableCookieJar) *KeepaliveCookieJar {
	k := &KeepaliveCookieJar{RefreshableCookieJar: jar}
	go k.Keepalive(ctx, refresh, true)
	return k
}

// Keepalive 立即开始保活 RefreshableCookieJar
func Keepalive(refresh time.Duration, jar RefreshableCookieJar) *KeepaliveCookieJar {
	return KeepaliveWithContext(context.Background(), refresh, jar)
}
