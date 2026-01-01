package req

import (
	"context"
	"errors"
	"math/rand"
	"net/http"
	"time"
)

// DoubleTicker 倍增计时器，初始重试间隔 1 秒，之后每次重试间隔翻倍，值为最大重试次数
type DoubleTicker int

func (t DoubleTicker) NextRetry(retried int) (time.Duration, bool) {
	return (1 << retried) * time.Second, retried < int(t)
}

// “试”不过三
var DefaultRetryTicker RetryTicker = DoubleTicker(2)

// ErrDuration 重试间隔时间非正
var ErrDuration = errors.New("req: time.Duration of ForeverTicker must be positive")

// ForeverTicker 永久计时器，每次都返回当前值的重试间隔
type ForeverTicker time.Duration

func (t ForeverTicker) NextRetry(int) (time.Duration, bool) {
	if t <= 0 {
		panic(ErrDuration)
	}
	return time.Duration(t), true
}

var _ RetryTicker = (*ForeverTicker)(nil)

// zeroTicker 零间隔计时器
type zeroTicker int

func (t zeroTicker) NextRetry(retried int) (time.Duration, bool) {
	return 0, retried < int(t)
}

// ZeroTicker 零间隔重试器，你应该知道自己在做什么、为什么这么做、为什么能这样做
//
//	var _ RetryTicker = ZeroTicker(2, "trust me!")
func ZeroTicker(maxRetries int, whySafe string) RetryTicker {
	return zeroTicker(maxRetries)
}

// FibonacciTicker 斐波那契计时器，前两项为第一次、第二次重试间隔时间
// 之后按照斐波那契规则返回新间隔时间，重试间隔时间超过第三项时终止
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

// RandomTicker 随机计时器，返回给入重试间隔之间的随机值
type RandomTicker [2]time.Duration

func (r RandomTicker) NextRetry(retried int) (delay time.Duration, ok bool) {
	if r[0] > r[1] {
		return time.Duration(rand.Int63n(int64(r[0]-r[1])) + int64(r[1])), true
	} else if r[0] < r[1] {
		return time.Duration(rand.Int63n(int64(r[1]-r[0])) + int64(r[0])), true
	} else {
		return r[0], true
	}
}

// RetryTransport 重试传输器
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
