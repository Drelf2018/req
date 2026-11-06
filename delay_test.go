package req

import (
	"testing"
	"time"
)

// TestDoubleDelayer 倍增：1s 起，每次翻倍，最多 d 次
func TestDoubleDelayer(t *testing.T) {
	d := DoubleDelayer(2)
	want := []struct {
		delay time.Duration
		ok    bool
	}{
		{1 * time.Second, true},
		{2 * time.Second, true},
		{4 * time.Second, false},
		{8 * time.Second, false},
	}
	for i, w := range want {
		got, ok := d.NextDelay(i)
		if got != w.delay || ok != w.ok {
			t.Errorf("times=%d → (%v,%v)，期望 (%v,%v)", i, got, ok, w.delay, w.ok)
		}
	}
	if _, ok := DefaultDelayer.NextDelay(2); ok {
		t.Error("DefaultDelayer 应当在第三次之前停止（试不过三）")
	}
}

// TestForeverDelayer 永远返回同一个间隔，且永不停止
func TestForeverDelayer(t *testing.T) {
	f := ForeverDelayer(1500 * time.Millisecond)
	for i := 0; i < 100; i++ {
		if got, ok := f.NextDelay(i); got != 1500*time.Millisecond || !ok {
			t.Fatalf("times=%d → (%v,%v)", i, got, ok)
		}
	}
}

// TestZeroDelayer 零间隔，按次数停止
func TestZeroDelayer(t *testing.T) {
	d := ZeroDelayer(2, "测试用，不会真的等")
	for i, want := range []bool{true, true, false} {
		if got, ok := d.NextDelay(i); got != 0 || ok != want {
			t.Errorf("times=%d → (%v,%v)，期望 (0,%v)", i, got, ok, want)
		}
	}
}

// TestFibonacciDelayer 前两项是种子，之后按斐波那契递增，超过上限终止；而且必须是纯函数
func TestFibonacciDelayer(t *testing.T) {
	d := FibonacciDelayer{time.Second, 2 * time.Second, 5 * time.Second}
	want := []struct {
		delay time.Duration
		ok    bool
	}{
		{1 * time.Second, true},
		{2 * time.Second, true},
		{3 * time.Second, true},
		{5 * time.Second, true},
		{8 * time.Second, false},
	}
	for i, w := range want {
		got, ok := d.NextDelay(i)
		if got != w.delay || ok != w.ok {
			t.Errorf("times=%d → (%v,%v)，期望 (%v,%v)", i, got, ok, w.delay, w.ok)
		}
	}
	// 同一个 times 反复问，结果必须一致：这里曾经改过自己的状态，导致并发 -race 报警
	for i := 0; i < 10; i++ {
		a, ao := d.NextDelay(3)
		b, bo := d.NextDelay(3)
		if a != b || ao != bo {
			t.Fatalf("times=3 两次调用不一致：(%v,%v) vs (%v,%v)", a, ao, b, bo)
		}
	}
}

// TestRandomDelayer 结果落在两个值之间（左闭右开），并且永不停止
func TestRandomDelayer(t *testing.T) {
	lo, hi := 10*time.Millisecond, 20*time.Millisecond
	for _, d := range []RandomDelayer{{lo, hi}, {hi, lo}} {
		for i := 0; i < 1000; i++ {
			got, ok := d.NextDelay(i)
			if !ok {
				t.Fatalf("%v 不该停止", d)
			}
			if got < lo || got >= hi {
				t.Fatalf("%v times=%d → %v，不在 [%v, %v) 内", d, i, got, lo, hi)
			}
		}
	}
}

// TestDelayFunc 函数包装也算延迟器
func TestDelayFunc(t *testing.T) {
	var d Delayer = DelayFunc(func(times int) (time.Duration, bool) {
		return time.Duration(times) * time.Millisecond, times < 3
	})
	for i, want := range []bool{true, true, true, false} {
		if got, ok := d.NextDelay(i); got != time.Duration(i)*time.Millisecond || ok != want {
			t.Errorf("times=%d → (%v,%v)，期望 (%v,%v)", i, got, ok, time.Duration(i)*time.Millisecond, want)
		}
	}
}

// TestDelayTicker 按延迟器发信，延迟器说不就自动关掉通道
func TestDelayTicker(t *testing.T) {
	ticker := NewDelayTicker(ZeroDelayer(2, "测试用"))
	defer ticker.Stop()
	var n int
	for range ticker.C {
		n++
	}
	if n != 2 {
		t.Errorf("收到 %d 个 tick，期望 2", n)
	}
}

// TestDelayTickerStop 手动 Stop 之后通道必须被关闭，不能挂住
func TestDelayTickerStop(t *testing.T) {
	ticker := NewDelayTicker(ForeverDelayer(time.Hour))
	ticker.Stop()
	ticker.Stop() // 重复 Stop 不该 panic
	select {
	case _, ok := <-ticker.C:
		if ok {
			t.Error("Stop 之后通道应当已关闭")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Stop 之后通道没有关闭")
	}
}

// TestWithDelay 先回调、再睡：第一次立刻发生，第二次在睡过之后，第三次之前延迟器到期
func TestWithDelay(t *testing.T) {
	start := time.Now()
	var times []int
	var delays []time.Duration
	var elapsed []time.Duration
	WithDelay(DelayFunc(func(n int) (time.Duration, bool) {
		return 50 * time.Millisecond, n < 2
	}))(func(n int, delay time.Duration) bool {
		times = append(times, n)
		delays = append(delays, delay)
		elapsed = append(elapsed, time.Since(start))
		return true
	})
	if len(times) != 2 {
		t.Fatalf("回调次数 = %d，期望 2（第三次之前延迟器到期）：%v", len(times), times)
	}
	if times[0] != 0 || times[1] != 1 {
		t.Errorf("times = %v", times)
	}
	if delays[0] != 50*time.Millisecond || delays[1] != 50*time.Millisecond {
		t.Errorf("delays = %v", delays)
	}
	if elapsed[0] > 40*time.Millisecond {
		t.Errorf("第一次回调应当立刻发生，实际等了 %v", elapsed[0])
	}
	if elapsed[1] < 50*time.Millisecond {
		t.Errorf("第二次回调应当在睡过之后，实际 %v", elapsed[1])
	}
}

// TestWithDelayStop 回调返回 false 就立刻结束，不再睡
func TestWithDelayStop(t *testing.T) {
	var n int
	WithDelay(ForeverDelayer(time.Hour))(func(times int, delay time.Duration) bool {
		n++
		return false
	})
	if n != 1 {
		t.Errorf("回调返回 false 之后不该继续，实际调用 %d 次", n)
	}
}
