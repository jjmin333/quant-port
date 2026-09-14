package ocr

import (
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// 세마포어가 실제로 동시 실행을 막는지 — 상한을 넘긴 순간이 한 번이라도 있으면 실패.
func TestAcquireBoundsConcurrency(t *testing.T) {
	orig := MaxConcurrency()
	t.Cleanup(func() { SetMaxConcurrency(orig) })

	const limit = 2
	SetMaxConcurrency(limit)

	var live, peak int64
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			release := acquire()
			defer release()

			n := atomic.AddInt64(&live, 1)
			for {
				p := atomic.LoadInt64(&peak)
				if n <= p || atomic.CompareAndSwapInt64(&peak, p, n) {
					break
				}
			}
			time.Sleep(2 * time.Millisecond)
			atomic.AddInt64(&live, -1)
		}()
	}
	wg.Wait()

	if got := atomic.LoadInt64(&peak); got > limit {
		t.Errorf("동시 실행이 상한을 넘었다: peak=%d limit=%d", got, limit)
	}
}

// 0은 무제한 — 롤백 경로가 실제로 막지 않는지.
func TestZeroDisablesLimit(t *testing.T) {
	orig := MaxConcurrency()
	t.Cleanup(func() { SetMaxConcurrency(orig) })

	SetMaxConcurrency(0)
	if MaxConcurrency() != 0 {
		t.Fatalf("MaxConcurrency()=%d, want 0", MaxConcurrency())
	}
	// 슬롯이 없으면 acquire가 절대 막히지 않아야 한다.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 50; i++ {
			release := acquire()
			defer release()
		}
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("무제한인데 acquire가 막혔다")
	}
}

// 환경변수가 없으면 컨테이너가 쓸 수 있는 코어 수를 따른다.
// Go 1.25는 cgroup CPU 상한에서 GOMAXPROCS를 잡으므로, 이것이 곧 "이 컨테이너의 코어 수"다.
func TestDefaultsToGOMAXPROCS(t *testing.T) {
	t.Setenv("OCR_MAX_CONCURRENCY", "")
	if got, want := envMaxConcurrency(), runtime.GOMAXPROCS(0); got != want {
		t.Errorf("기본값 %d, want %d", got, want)
	}
}

func TestEnvOverride(t *testing.T) {
	t.Setenv("OCR_MAX_CONCURRENCY", "3")
	if got := envMaxConcurrency(); got != 3 {
		t.Errorf("OCR_MAX_CONCURRENCY=3 인데 %d", got)
	}
	// 쓰레기 값은 기본값으로 되돌아간다 — 오타 하나가 상한을 없애면 안 된다.
	_ = os.Setenv("OCR_MAX_CONCURRENCY", "무제한")
	if got, want := envMaxConcurrency(), runtime.GOMAXPROCS(0); got != want {
		t.Errorf("잘못된 값에서 %d, want %d", got, want)
	}
}
