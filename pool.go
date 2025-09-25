package req

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

type CookieState int

const (
	Unverified CookieState = iota // 未验证
	Verifying                     // 验证中
	Verified                      // 已验证
	Invalid                       // 已失效
)

type RefreshableCookieJar interface {
	http.CookieJar

	// 检测 Cookie 是否失效
	IsValid(context.Context) bool

	// 刷新 Cookie
	Refresh(context.Context) error
}

type CookieItem struct {
	// 具体实例
	RefreshableCookieJar

	// 当前状态
	state CookieState

	// 刷新锁
	m sync.Mutex
}

func (c *CookieItem) State() CookieState {
	return c.state
}

// 验证 Cookie
func (c *CookieItem) Verify(ctx context.Context) error {
	c.m.Lock()
	defer c.m.Unlock()

	c.state = Verifying
	if c.IsValid(ctx) {
		c.state = Verified
		return nil
	}

	err := ctx.Err()
	if err != nil {
		c.state = Invalid
		return fmt.Errorf("req: verify cookie failed: %w", err)
	}

	err = c.Refresh(ctx)
	if err != nil {
		c.state = Invalid
		return fmt.Errorf("req: refresh cookie failed: %w", err)
	}

	c.state = Verified
	return nil
}

func (c *CookieItem) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.RefreshableCookieJar)
}

var _ json.Marshaler = (*CookieItem)(nil)

func (c *CookieItem) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &c.RefreshableCookieJar)
}

var _ json.Unmarshaler = (*CookieItem)(nil)

type CookiePool struct {
	// Cookie 失效钩子
	InvalidCookieHook func(cookie *CookieItem, err error)

	// Cookie 切片
	items []*CookieItem

	// 检测刷新的间隔
	refresh time.Duration

	// 读写锁
	rw sync.RWMutex
}

// 获取全部 Cookie
func (p *CookiePool) All() []*CookieItem {
	p.rw.RLock()
	defer p.rw.RUnlock()
	return p.items
}

// 获取随机 Cookie
func (p *CookiePool) Random() *CookieItem {
	p.rw.RLock()
	defer p.rw.RUnlock()
	verifiedItems := make([]*CookieItem, 0, len(p.items))
	for _, item := range p.items {
		if item.state == Verified {
			verifiedItems = append(verifiedItems, item)
		}
	}
	if len(verifiedItems) == 0 {
		return nil
	}
	return verifiedItems[rand.Intn(len(verifiedItems))]
}

// 定时验证
func (p *CookiePool) Verify(ctx context.Context, item *CookieItem, refresh time.Duration) {
	ticker := time.NewTicker(refresh)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			item.state = Invalid
			return
		case <-ticker.C:
			err := item.Verify(ctx)
			if err != nil {
				if p.InvalidCookieHook != nil {
					p.InvalidCookieHook(item, err)
				}
				return
			}
		}
	}
}

// 添加 CookieJar
func (p *CookiePool) AddWithRefresh(jar RefreshableCookieJar, refresh time.Duration) context.CancelFunc {
	cookie := &CookieItem{RefreshableCookieJar: jar}
	ctx, cancel := context.WithCancel(context.Background())
	// 开始定时验证
	go p.Verify(ctx, cookie, refresh)
	// 保存新 Cookie
	p.rw.Lock()
	defer p.rw.Unlock()
	// 覆盖失效
	for idx, item := range p.items {
		if item.state == Invalid {
			p.items[idx] = cookie
			return cancel
		}
	}
	// 添加新值
	p.items = append(p.items, cookie)
	return cancel
}

// 添加 CookieJar
func (p *CookiePool) Add(jar RefreshableCookieJar) context.CancelFunc {
	return p.AddWithRefresh(jar, p.refresh)
}

func NewCookiePool(refresh time.Duration) *CookiePool {
	return &CookiePool{refresh: refresh}
}
