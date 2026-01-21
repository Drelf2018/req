package req

import (
	"errors"
	"math/rand"
	"time"
)

// RetryTicker 重试计时器
type RetryTicker interface {
	// NextRetry 用于计算下次重试前需要等待的时间，入参为已重试次数，返回值为等待时间以及是否继续重试
	NextRetry(retried int) (delay time.Duration, ok bool)
}

// RetryFunc 重试函数
type RetryFunc func(retried int) (delay time.Duration, ok bool)

func (r RetryFunc) NextRetry(retried int) (delay time.Duration, ok bool) { return r(retried) }

var _ RetryTicker = (*RetryFunc)(nil)

// DoubleTicker 倍增计时器，初始重试间隔 1 秒，之后每次重试间隔翻倍，值为最大重试次数
type DoubleTicker int

func (t DoubleTicker) NextRetry(retried int) (time.Duration, bool) {
	return (1 << retried) * time.Second, retried < int(t)
}

// DefaultRetryTicker 默认计时器，“试”不过三
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

// Ticker 到达重试计时器返回的下次重试时间时，会发送当前时间到通道。当计时器不再重试时，会自动关闭通道
type Ticker struct {
	C    <-chan time.Time
	stop chan struct{}
}

// Stop 手动停止重试计时器
func (t *Ticker) Stop() {
	select {
	case <-t.stop:
	default:
		close(t.stop)
	}
}

func (t *Ticker) run(retry RetryTicker, out chan time.Time) {
	defer close(out)
	// 内部定时器，用来产生信号
	var timer *time.Timer
	for retried := 0; ; retried++ {
		// 循环获取延迟时间
		delay, ok := retry.NextRetry(retried)
		if !ok {
			return
		}
		// 初始化定时器或重置定时器
		if timer == nil {
			timer = time.NewTimer(delay)
		} else {
			timer.Reset(delay)
		}
		// 监听定时器或停止通道触发
		select {
		case now := <-timer.C:
			// 将定时器触发时间阻塞转发给用户
			select {
			case out <- now:
			case <-t.stop:
				return
			}
		case <-t.stop:
			// 用户主动关闭
			if !timer.Stop() {
				<-timer.C
			}
			return
		}
	}
}

// NewTicker 创建并立即启动 Ticker
func NewTicker(retry RetryTicker) *Ticker {
	out := make(chan time.Time) // 转发通道，不能将内部定时器的通道直接导出
	t := &Ticker{C: out, stop: make(chan struct{})}
	go t.run(retry, out)
	return t
}
