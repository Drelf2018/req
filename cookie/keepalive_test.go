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

// TestRunSendsToOnError cron 场景：错误走 OnError
func TestRunSendsToOnError(t *testing.T) {
	var got error
	k := &KeepaliveCookieJar{
		Refresher: &fakeRefresher{validErr: errors.New("boom")},
		OnError:   func(err error) { got = err },
	}
	k.Run()
	if got == nil {
		t.Fatal("OnError 没被调用")
	}
}
