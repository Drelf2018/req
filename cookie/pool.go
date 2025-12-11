package cookie

import (
	"context"
	"math/rand"
	"sync"
	"time"
)

// Pool 是自动保活的 CookieJar 的池，可以随机取出已验证的 CookieJar 使用
type Pool struct {
	// 检测 Cookie 是否有效的时间间隔
	Refresh time.Duration

	// 保活过程中出现错误时自动调用
	OnError func(k *KeepaliveCookieJar, err error)

	rw sync.RWMutex

	ctx context.Context

	cancel context.CancelFunc

	cookies []*KeepaliveCookieJar
}

// Add 添加 Cookie
func (p *Pool) Add(jar RefreshableCookieJar) *KeepaliveCookieJar {
	p.rw.Lock()
	defer p.rw.Unlock()
	if p.cancel == nil {
		p.ctx, p.cancel = context.WithCancel(context.Background())
	}
	k := &KeepaliveCookieJar{RefreshableCookieJar: jar, OnError: p.OnError}
	p.cookies = append(p.cookies, k)
	go k.Keepalive(p.ctx, p.Refresh)
	return k
}

// Random 获取随机 Cookie
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

// Get 获取有效下标的 Cookie ，不指定下标时返回所有 Cookie
func (p *Pool) Get(index ...int) []*KeepaliveCookieJar {
	p.rw.RLock()
	defer p.rw.RUnlock()
	if len(index) == 0 {
		return p.cookies
	}
	cookies := make([]*KeepaliveCookieJar, len(index))
	for i, idx := range index {
		if 0 <= idx && idx < len(p.cookies) {
			cookies[i] = p.cookies[idx]
		}
	}
	return cookies
}

// Stop 停止所有 Cookie 保活
func (p *Pool) Stop() {
	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}
}
