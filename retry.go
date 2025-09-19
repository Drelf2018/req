package req

import (
	"context"
	"errors"
	"net/http"
	"time"
)

// 初始重试时间间隔 1 秒 之后每次重试时间间隔翻倍
//
// 值表示最大重试次数
type DoubleTicker int

func (t DoubleTicker) NextRetry(retried int) (time.Duration, bool) {
	return (1 << retried) * time.Second, retried < int(t)
}

// “试”不过三
var DefaultRetryTicker RetryTicker = DoubleTicker(2)

var ErrDuration = errors.New("req: time.Duration of ForeverTicker must be positive")

// 永久重试器
type ForeverTicker time.Duration

func (t ForeverTicker) NextRetry(int) (time.Duration, bool) {
	if t <= 0 {
		panic(ErrDuration)
	}
	return time.Duration(t), true
}

var _ RetryTicker = (*ForeverTicker)(nil)

type zeroTicker int

func (t zeroTicker) NextRetry(retried int) (time.Duration, bool) {
	return 0, retried < int(t)
}

// 零间隔重试器
//
// 你应该知道自己在做什么、为什么这么做、为什么能这样做
//
//	var _ RetryTicker = ZeroTicker(2, "trust me!")
func ZeroTicker(maxRetries int, whySafe string) RetryTicker {
	return zeroTicker(maxRetries)
}

// 斐波那契重试器
//
// 前两项为第一次、第二次重试间隔时间，之后按照斐波那契规则返回新间隔时间，第三项为最大单次重试间隔时间
type FibonacciTicker [3]time.Duration

func (t *FibonacciTicker) NextRetry(retried int) (time.Duration, bool) {
	switch retried {
	case 0, 1:
		return t[retried], t[retried] <= t[2]
	default:
		t[0], t[1] = t[1], t[0]+t[1]
		return t[1], t[1] <= t[2]
	}
}

// 重试传输器
type RetryTransport struct {
	http.RoundTripper
	RetryTicker
}

func (t *RetryTransport) RoundTrip(r *http.Request) (resp *http.Response, err error) {
	resp, err = t.RoundTripper.RoundTrip(r)
	for i := 0; err != nil; i++ {
		d, ok := t.NextRetry(i)
		if !ok {
			break
		}
		time.Sleep(d)
		resp, err = t.RoundTripper.RoundTrip(r)
	}
	return
}

var _ http.RoundTripper = (*RetryTransport)(nil)

func NewRetryTransport(ticker RetryTicker) *RetryTransport {
	return &RetryTransport{http.DefaultTransport, ticker}
}

// WithRetry 可以将重试器转换成一个定时返回当前重试次数的通道
//
//	func TestRetry(t *testing.T) {
//		resp, err := GetStatus()
//		if err != nil {
//			c, cancel := req.WithRetry(req.DefaultRetryTicker)
//			for range c {
//				resp, err = GetStatus()
//				if err == nil {
//					cancel()
//				}
//			}
//		}
//		t.Log(resp.Status)
//	}
func WithRetry(r RetryTicker) (<-chan int, context.CancelFunc) {
	c := make(chan int)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		timer := time.NewTimer(time.Hour)
		for i := 0; ; i++ {
			if d, ok := r.NextRetry(i); ok {
				timer.Reset(d)
			} else {
				timer.Stop()
				close(c)
				return
			}
			select {
			case <-ctx.Done():
				timer.Stop()
				close(c)
				return
			case <-timer.C:
				c <- i
			}
		}
	}()
	return c, cancel
}
