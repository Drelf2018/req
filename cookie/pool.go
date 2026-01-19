package cookie

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Drelf2018/req"
)

// State 是 http.CookieJar 的验证状态
type State int32

const (
	StateUnverified State = iota // 未验证
	StateVerifying               // 验证中
	StateVerified                // 已验证
	StateInvalid                 // 已失效
)

// Refresher 刷新器，用于检测和刷新 http.CookieJar
type Refresher interface {
	// IsValid 检测 http.CookieJar 是否有效
	IsValid(context.Context, http.CookieJar) (bool, error)

	// Refresh 刷新 http.CookieJar
	Refresh(context.Context, http.CookieJar) error
}

// ForcedRefresher 强制刷新器，每次检测时都认定 http.CookieJar 已失效，并将自身用于刷新
type ForcedRefresher func(context.Context, http.CookieJar) error

func (f ForcedRefresher) IsValid(context.Context, http.CookieJar) (bool, error) { return false, nil }

func (f ForcedRefresher) Refresh(ctx context.Context, jar http.CookieJar) error { return f(ctx, jar) }

var _ Refresher = (*ForcedRefresher)(nil)

// Verify 检测 http.CookieJar 是否有效，如果已经失效会进行一次刷新
func Verify(ctx context.Context, refresher Refresher, jar http.CookieJar) (State, error) {
	valid, err := refresher.IsValid(ctx, jar)
	if err != nil {
		return StateUnverified, fmt.Errorf("req/cookie: failed to verify: %w", err)
	}
	if valid {
		return StateVerified, nil
	}
	err = refresher.Refresh(ctx, jar)
	if err != nil {
		return StateInvalid, fmt.Errorf("req/cookie: failed to refresh: %w", err)
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

	// 上次验证成功的时间
	lastVerified time.Time

	// 主动取消保活
	cancel context.CancelFunc

	// 当前状态
	state int32
}

// State 获取 http.CookieJar 的验证状态
func (k *KeepaliveCookieJar) State() State {
	return State(atomic.LoadInt32(&k.state))
}

// SinceLastVerified 获取距离上次验证成功时过去的时间
func (k *KeepaliveCookieJar) SinceLastVerified() time.Duration {
	return time.Since(k.lastVerified)
}

// Verify 携带上下文检测 http.CookieJar 是否有效
func (k *KeepaliveCookieJar) Verify(ctx context.Context) {
	atomic.StoreInt32(&k.state, int32(StateVerifying))
	state, err := Verify(ctx, k.Refresher, k.CookieJar)
	atomic.StoreInt32(&k.state, int32(state))
	if err != nil {
		if k.OnError != nil {
			k.OnError(err)
		}
	} else if state == StateVerified {
		k.lastVerified = time.Now()
	}
}

// Run 检测 http.CookieJar 是否有效
func (k *KeepaliveCookieJar) Run() {
	k.Verify(context.Background())
}

// Keepalive 自动保活 http.CookieJar ，延时结束后进行一次检测，如果不传入则立即执行一次检测，传入非正数则不预先检测
func (k *KeepaliveCookieJar) Keepalive(ctx context.Context, retry req.RetryTicker, delay ...time.Duration) {
	// 可以主动取消
	ctx, k.cancel = context.WithCancel(ctx)
	defer k.cancel()
	// 延时结束后进行一次检测，如果不传入则立即执行一次检测
	var totalDelay time.Duration
	for _, d := range delay {
		totalDelay += d
	}
	if totalDelay > 0 {
		timer := time.NewTimer(totalDelay)
		select {
		case <-ctx.Done():
			if ctx.Err() != nil && k.OnError != nil {
				k.OnError(ctx.Err())
			}
			timer.Stop()
			return
		case <-timer.C:
			k.Verify(ctx)
			timer.Stop()
		}
	} else if totalDelay == 0 {
		k.Verify(ctx)
	}
	// 每当重试器触发时进行一次检测
	ticker := req.NewTicker(retry)
	for {
		select {
		case <-ctx.Done():
			if ctx.Err() != nil && k.OnError != nil {
				k.OnError(ctx.Err())
			}
			ticker.Stop()
			return
		case <-ticker.C:
			k.Verify(ctx)
		}
	}
}

// StopKeepalive 主动取消保活 http.CookieJar
func (k *KeepaliveCookieJar) StopKeepalive() {
	if k.cancel != nil {
		k.cancel()
	}
}

// KeepaliveWithContext 携带上下文立即开始保活 http.CookieJar
func KeepaliveWithContext(ctx context.Context, jar http.CookieJar, refresher Refresher, retry req.RetryTicker, delay ...time.Duration) *KeepaliveCookieJar {
	k := &KeepaliveCookieJar{CookieJar: jar, Refresher: refresher}
	go k.Keepalive(ctx, retry, delay...)
	return k
}

// Keepalive 立即开始保活 http.CookieJar
func Keepalive(jar http.CookieJar, refresher Refresher, retry req.RetryTicker, delay ...time.Duration) *KeepaliveCookieJar {
	return KeepaliveWithContext(context.Background(), jar, refresher, retry, delay...)
}

// Pool 是自动保活的 http.CookieJar 的池，可以获取随机已验证的实例
type Pool struct {
	// 检测 http.CookieJar 是否有效的时间间隔
	Refresh time.Duration

	// 错误上报
	OnError func(error)

	rw sync.RWMutex

	ctx context.Context

	cancel context.CancelFunc

	cookies []*KeepaliveCookieJar
}

// Add 添加 http.CookieJar 并且开始保活
func (p *Pool) Add(jar http.CookieJar, refresher Refresher, delay ...time.Duration) *KeepaliveCookieJar {
	p.rw.Lock()
	defer p.rw.Unlock()
	if p.cancel == nil {
		p.ctx, p.cancel = context.WithCancel(context.Background())
	}
	k := &KeepaliveCookieJar{CookieJar: jar, Refresher: refresher, OnError: p.OnError}
	p.cookies = append(p.cookies, k)
	go k.Keepalive(p.ctx, req.ForeverTicker(p.Refresh), delay...)
	return k
}

// Random 获取随机已验证的 http.CookieJar
func (p *Pool) Random() *KeepaliveCookieJar {
	p.rw.RLock()
	defer p.rw.RUnlock()
	verified := make([]*KeepaliveCookieJar, 0, len(p.cookies))
	for _, cookie := range p.cookies {
		if cookie.State() == StateVerified {
			verified = append(verified, cookie)
		}
	}
	if len(verified) == 0 {
		return nil
	}
	return verified[rand.Intn(len(verified))]
}

// Get 获取有效下标的 http.CookieJar ，不指定下标时返回全部 http.CookieJar
func (p *Pool) Get(index ...int) []*KeepaliveCookieJar {
	p.rw.RLock()
	defer p.rw.RUnlock()
	if len(index) == 0 {
		return append([]*KeepaliveCookieJar{}, p.cookies...)
	}
	cookies := make([]*KeepaliveCookieJar, len(index))
	for i, idx := range index {
		if 0 <= idx && idx < len(p.cookies) {
			cookies[i] = p.cookies[idx]
		}
	}
	return cookies
}

// Stop 停止所有 http.CookieJar 保活
func (p *Pool) Stop() {
	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}
}
