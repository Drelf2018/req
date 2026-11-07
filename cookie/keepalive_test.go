package cookie

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

// fakeRefresher 同时实现 Refresher 和 Validator，用来驱动四种转移
type fakeRefresher struct {
	valid    bool
	validErr error
	refresh  error
	calls    int
}

func (f *fakeRefresher) Valid(context.Context, http.CookieJar) (bool, error) {
	return f.valid, f.validErr
}

func (f *fakeRefresher) Refresh(context.Context, http.CookieJar) error {
	f.calls++
	return f.refresh
}

// fakeLogger 记录错误上报，用来断言 Logger 有没有被调用。
// Logger 的签名取自 log/slog 的 ErrorContext(ctx, msg, args...)
type fakeLogger struct {
	msgs  []string
	attrs [][]any
}

func (f *fakeLogger) ErrorContext(_ context.Context, msg string, args ...any) {
	f.msgs = append(f.msgs, msg)
	f.attrs = append(f.attrs, args)
}

func TestVerifyStates(t *testing.T) {
	boom := errors.New("boom")
	cases := []struct {
		name      string
		r         Refresher
		want      State
		wantErr   string
		refreshed bool
	}{
		{
			name: "检测通过，不刷新",
			r:    &fakeRefresher{valid: true},
			want: StateVerified,
		},
		{
			name:    "检测报错",
			r:       &fakeRefresher{validErr: boom},
			want:    StateFailed,
			wantErr: "cannot validate jar: boom",
		},
		{
			name:      "检测说无效，刷新成功",
			r:         &fakeRefresher{valid: false},
			want:      StateVerified,
			refreshed: true,
		},
		{
			name:      "检测说无效，刷新失败",
			r:         &fakeRefresher{valid: false, refresh: boom},
			want:      StateFailed,
			wantErr:   "cannot refresh jar: boom",
			refreshed: true,
		},
		{
			// 没实现 Validator 就直接刷新，等价于旧的 ForcedRefresher
			name:      "没有检测器，直接刷新",
			r:         RefreshFunc(func(context.Context, http.CookieJar) error { return nil }),
			want:      StateVerified,
			refreshed: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			state, err := Verify(context.Background(), c.r, nil)
			if state != c.want {
				t.Errorf("state = %v，期望 %v", state, c.want)
			}
			if c.wantErr == "" {
				if err != nil {
					t.Errorf("不该报错: %v", err)
				}
			} else if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("err = %v，期望包含 %q", err, c.wantErr)
			}
			if f, ok := c.r.(*fakeRefresher); ok {
				got := f.calls > 0
				if got != c.refreshed {
					t.Errorf("刷新被调用 %d 次，期望被调用 = %v", f.calls, c.refreshed)
				}
			}
		})
	}
}

// TestKeepaliveJarVerify 包一层的方法要更新状态和时间
func TestKeepaliveJarVerify(t *testing.T) {
	k := &KeepaliveCookieJar{Refresher: &fakeRefresher{valid: true}}
	if err := k.Verify(context.Background()); err != nil {
		t.Fatal(err)
	}
	if k.State() != StateVerified {
		t.Errorf("State = %v", k.State())
	}
	if d := k.SinceVerified(); d < 0 || d > 5*time.Second {
		t.Errorf("SinceVerified = %v", d)
	}
	k.Refresher = &fakeRefresher{validErr: errors.New("x")}
	if err := k.Verify(context.Background()); err == nil {
		t.Fatal("应当报错")
	}
	if k.State() != StateFailed {
		t.Errorf("State = %v", k.State())
	}
}

// TestVerifyLogs 配置了 Logger 时，检测与刷新失败都要上报一条
func TestVerifyLogs(t *testing.T) {
	boom := errors.New("boom")
	cases := []struct {
		name    string
		r       Refresher
		wantMsg string
	}{
		{
			name:    "检测报错",
			r:       &fakeRefresher{validErr: boom},
			wantMsg: "cannot validate jar",
		},
		{
			name:    "刷新报错",
			r:       RefreshFunc(func(context.Context, http.CookieJar) error { return boom }),
			wantMsg: "cannot refresh jar",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			l := &fakeLogger{}
			k := &KeepaliveCookieJar{Refresher: c.r, Logger: l}
			err := k.Verify(context.Background())
			if err == nil {
				t.Fatal("应当报错")
			}
			// 错误本身仍要能被调用方识别，不能只剩日志
			if !errors.Is(err, boom) {
				t.Errorf("err = %v，期望能 errors.Is 到 boom", err)
			}
			var v *VerifyError
			if !errors.As(err, &v) {
				t.Fatalf("err = %T，期望 *VerifyError", err)
			}
			if len(l.msgs) != 1 {
				t.Fatalf("上报了 %d 条，期望 1 条", len(l.msgs))
			}
			if l.msgs[0] != c.wantMsg {
				t.Errorf("msg = %q，期望 %q", l.msgs[0], c.wantMsg)
			}
			if len(l.attrs[0]) != 2 || l.attrs[0][0] != "error" {
				t.Fatalf("attrs = %v，期望 [error, err]", l.attrs[0])
			}
			if got, ok := l.attrs[0][1].(error); !ok || !errors.Is(got, boom) {
				t.Errorf("attrs[1] = %v，期望 boom", l.attrs[0][1])
			}
		})
	}
}

// TestRunLogs cron 场景：Run 自己吞掉错误，但要把错误交给 Logger
func TestRunLogs(t *testing.T) {
	l := &fakeLogger{}
	k := &KeepaliveCookieJar{
		Refresher: &fakeRefresher{validErr: errors.New("boom")},
		Logger:    l,
	}
	k.Run()
	if len(l.msgs) != 1 {
		t.Fatalf("上报了 %d 条，期望 1 条", len(l.msgs))
	}
	if k.State() != StateFailed {
		t.Errorf("State = %v，期望 StateFailed", k.State())
	}
}

// TestRunWithoutLogger 没有 Logger 时静默返回，但不能 panic
func TestRunWithoutLogger(t *testing.T) {
	k := &KeepaliveCookieJar{Refresher: &fakeRefresher{validErr: errors.New("boom")}}
	k.Run()
	k.RunWithContext(context.Background())
	if k.State() != StateFailed {
		t.Errorf("State = %v，期望 StateFailed", k.State())
	}
}

// TestRunE 不传上下文也要返回错误
func TestRunE(t *testing.T) {
	k := &KeepaliveCookieJar{Refresher: &fakeRefresher{valid: true}}
	if err := k.RunE(); err != nil {
		t.Fatal(err)
	}
	if k.State() != StateVerified {
		t.Errorf("State = %v，期望 StateVerified", k.State())
	}
	k.Refresher = &fakeRefresher{validErr: errors.New("boom")}
	if err := k.RunE(); err == nil {
		t.Fatal("应当报错")
	}
}
