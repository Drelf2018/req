package req

import (
	"errors"
	"time"
)

// 初始重试时间 1 秒 之后每次重试时间翻倍
//
// 值表示最大重试次数
type DoubleTimer int

func (t DoubleTimer) NextRetry(num int) (time.Duration, bool) {
	if num >= int(t) {
		return 0, false
	}
	return (1 << num) * time.Second, true
}

// 事不过三
var DefaultRetryTimer RetryTimer = DoubleTimer(2)

var ErrDuration = errors.New("req: time.Duration must be positive")

// 永久重试器
type ForeverTimer time.Duration

func (t ForeverTimer) NextRetry(int) (time.Duration, bool) {
	if t <= 0 {
		panic(ErrDuration)
	}
	return time.Duration(t), true
}

var _ RetryTimer = (*ForeverTimer)(nil)

type zeroTimer int

func (t zeroTimer) NextRetry(num int) (time.Duration, bool) {
	return 0, num < int(t)
}

// 零间隔重试器
//
// 你应该知道自己在做什么、为什么这么做、为什么能这样做
//
//	var _ RetryTimer = ZeroTimer(2, "trust me")
func ZeroTimer(maxRetry int, whySafe string) RetryTimer {
	return zeroTimer(maxRetry)
}
