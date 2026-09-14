package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
)

// fetchURL은 **예산 검사와 기록을 여기 한 곳에서** 한다.
//
// 예전에는 `fetchURLHTTP`만 `Check`/`Record`를 하고 curl 폴백은 아무것도 안 했다.
// 그런데 Windows에서는 Go의 TLS 협상이 DART와 실패해서(`isDartTLSFailure`) **모든
// 호출이 curl 경로로 간다** — 즉 개발 환경의 DART 호출이 통째로 예산에 안 잡혔다.
//
// 실측 2026-09-02: 재적재 배치로 2만 회 넘게 부르고 DART가 `020 사용한도 초과`를
// 돌려주는 동안, 우리 계기판은 `used_day=0 / headroom=18000`이었다. 그래서
// `backfillMaxSymbols` 가드가 아무것도 못 막았다 — 계기가 0을 가리키는데 막을
// 방법이 없다.
func (d *DartClient) fetchURL(ctx context.Context, u string) ([]byte, error) {
	// 간격은 **기다려서** 지킨다. `Check`에 그냥 맡기면 두 번째 호출부터
	// `ErrPacing`이 나고, 호출자는 그것을 「그 분기 미제출」로 읽어 조용히 건너뛴다
	// — 재무 적재는 종목당 20회 넘게 연달아 부르므로 대부분이 그렇게 사라진다.
	if err := d.waitForBudget(ctx); err != nil {
		return nil, err
	}
	body, err := d.fetchURLHTTP(ctx, u)
	if err != nil {
		if !isDartTLSFailure(err) {
			return nil, err
		}
		body, err = fetchURLViaCurl(ctx, u)
		if err != nil {
			return nil, err
		}
	}
	if d.budget != nil {
		d.budget.Record(ctx, apibudget.ProviderDART)
	}
	return body, nil
}

// fetchURLHTTP는 전송만 한다 — 예산 검사·기록은 fetchURL이 맡는다.
func (d *DartClient) fetchURLHTTP(ctx context.Context, u string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	res, err := d.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusTooManyRequests {
		if d.budget != nil {
			d.budget.BlockFor(ctx, apibudget.ProviderDART, apibudget.ParseRetryAfter(res.Header.Get("Retry-After")))
		}
		body, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return nil, fmt.Errorf("dart http %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return nil, fmt.Errorf("dart http %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

// fetchURLViaCurl shells out to curl when Go's TLS client fails to negotiate
// with opendart.fss.or.kr — observed on both native Windows and inside the
// Linux Docker container (curl's TLS stack succeeds where crypto/tls doesn't).
func fetchURLViaCurl(ctx context.Context, u string) ([]byte, error) {
	bin := "curl"
	if runtime.GOOS == "windows" {
		bin = "curl.exe"
	}
	cmd := exec.CommandContext(ctx, bin, "-sS", "-f", "-L", u)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("dart curl: %w", err)
	}
	return out, nil
}

func isDartTLSFailure(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "tls: handshake failure") ||
		strings.Contains(msg, "tls: protocol version") ||
		strings.Contains(msg, "certificate signed by unknown authority")
}

// waitForBudget는 예산 승인이 날 때까지 기다린다.
//
// `Check`는 판정만 한다 — 간격이 안 지났으면 `ErrPacing`을 낼 뿐이다. 여기서
// 그 판정을 **기다림**으로 바꾼다(`MinInterval` 주석이 말하는 「배치가 스스로
// 페이스를 잡는다」가 이것이다). 소진(`ErrExhausted`)은 기다려도 안 풀리므로
// 그대로 올린다.
//
// 예산 계정(Record)을 고치고 나서야 드러난 문제다. 그전에는 curl 폴백이 Record를
// 안 해 `last`가 갱신되지 않았고, 그래서 Check가 늘 통과했다 — 간격 제한이
// 사실상 없었던 셈이고 그게 하루 한도를 태운 이유다.
func (d *DartClient) waitForBudget(ctx context.Context) error {
	if d == nil || d.budget == nil {
		return nil
	}
	interval := d.budget.MinInterval(apibudget.ProviderDART)
	if interval <= 0 {
		interval = time.Second
	}
	d.paceMu.Lock()
	defer d.paceMu.Unlock()
	// 한 호출이 간격을 여러 번 기다릴 일은 없다. 상한을 두는 것은 시계가
	// 어긋나 영원히 도는 일을 막기 위해서다.
	for attempt := 0; attempt < 4; attempt++ {
		err := d.budget.Check(ctx, apibudget.ProviderDART)
		if err == nil {
			return nil
		}
		if !errors.Is(err, apibudget.ErrPacing) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
	return d.budget.Check(ctx, apibudget.ProviderDART)
}
