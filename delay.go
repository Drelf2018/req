package req

import (
	"math/rand"
	"time"
)

// Delayer 延迟器
type Delayer interface {
	// NextDelay 计算下次延迟前需要等待的时长，入参为已延迟次数，返回值为等待时长以及是否继续延迟
	NextDelay(times int) (delay time.Duration, ok bool)
}

// DelayFunc 延迟函数
type DelayFunc func(times int) (delay time.Duration, ok bool)

func (r DelayFunc) NextDelay(times int) (time.Duration, bool) {
	return r(times)
}

var _ Delayer = (*DelayFunc)(nil)

// DoubleDelayer 倍增延迟器，初始延迟间隔 1 秒，之后每次延迟间隔翻倍，值为最大延迟次数
type DoubleDelayer int

func (d DoubleDelayer) NextDelay(times int) (time.Duration, bool) {
	return (1 << times) * time.Second, times < int(d)
}

// DefaultDelayer 默认延迟器，“试”不过三
var DefaultDelayer Delayer = DoubleDelayer(2)

// ForeverDelayer 永久延迟器，每次都返回当前值的延迟间隔
type ForeverDelayer time.Duration

func (f ForeverDelayer) NextDelay(i int) (time.Duration, bool) {
	return time.Duration(f), true
}

var _ Delayer = ForeverDelayer(0)

// zeroDelayer 零间隔延迟器
type zeroDelayer int

func (t zeroDelayer) NextDelay(times int) (time.Duration, bool) {
	return 0, times < int(t)
}

// ZeroDelayer 零间隔延迟器，你应该知道自己在做什么、为什么这么做、为什么能这样做
//
//	var _ RetryDelayer = ZeroDelayer(2, "trust me!")
func ZeroDelayer(maxRetries int, whySafe string) Delayer {
	return zeroDelayer(maxRetries)
}

// FibonacciDelayer 斐波那契延迟器，前两项为第一次、第二次延迟时长
// 之后按照斐波那契规则返回新延迟时长，延迟时长超过第三项时终止
type FibonacciDelayer [3]time.Duration

func (t *FibonacciDelayer) NextDelay(times int) (time.Duration, bool) {
	switch times {
	case 0, 1:
		return t[times], t[times] <= t[2]
	default:
		t[0], t[1] = t[1], t[0]+t[1]
		return t[1], t[1] <= t[2]
	}
}

var _ Delayer = (*FibonacciDelayer)(nil)

// RandomDelayer 随机延迟器，返回给入延迟间隔之间的随机值
type RandomDelayer [2]time.Duration

func (r RandomDelayer) NextDelay(times int) (delay time.Duration, ok bool) {
	if r[0] > r[1] {
		return time.Duration(rand.Int63n(int64(r[0]-r[1])) + int64(r[1])), true
	} else if r[0] < r[1] {
		return time.Duration(rand.Int63n(int64(r[1]-r[0])) + int64(r[0])), true
	} else {
		return r[0], true
	}
}

var _ Delayer = (*RandomDelayer)(nil)

// DelayTicker 到达延迟延迟器返回的下次延迟时长时，会发送当前时间到通道。当延迟器不再延迟时，会自动关闭通道
type DelayTicker struct {
	C    <-chan time.Time
	stop chan struct{}
}

// Stop 手动停止延迟延迟器
func (d *DelayTicker) Stop() {
	select {
	case <-d.stop:
	default:
		close(d.stop)
	}
}

func (d *DelayTicker) run(delayer Delayer, out chan time.Time) {
	defer close(out)
	// 内部定时器，用来产生信号
	var timer *time.Timer
	// 已延迟次数
	for times := 0; ; {
		// 循环获取延迟时长
		delay, ok := delayer.NextDelay(times)
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
			// 将定时器触发时间非阻塞转发给用户，如果用户接收了则将计数器自增
			select {
			case out <- now:
				times++
			default:
			}
		case <-d.stop:
			// 用户主动关闭
			if !timer.Stop() {
				<-timer.C
			}
			return
		}
	}
}

// NewDelayTicker 创建并立即启动 Ticker
func NewDelayTicker(delayer Delayer) *DelayTicker {
	// 不能将内部定时器的通道直接导出
	out := make(chan time.Time, 1)
	t := &DelayTicker{C: out, stop: make(chan struct{})}
	go t.run(delayer, out)
	return t
}

// EOD 延迟器结束符 End Of Delayer
const EOD time.Duration = -1

// WithDelay 迭代一个延迟器，会立即进行一次迭代，在延迟后进行下次迭代。迭代值为已延迟次数和接下来延迟时长，延迟时长为负表示不再进行下次迭代
func WithDelay(delayer Delayer) func(yield func(times int, delay time.Duration) bool) {
	return func(yield func(times int, delay time.Duration) bool) {
		for times := 0; ; times++ {
			delay, ok := delayer.NextDelay(times)
			if !ok {
				yield(times, EOD)
				return
			}
			if !yield(times, delay) {
				return
			}
			time.Sleep(delay)
		}
	}
}

// WithDefaultDelay 迭代一个默认延迟器
var WithDefaultDelay = WithDelay(DefaultDelayer)
