package req

import (
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
