package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"runtime"
	"strings"

	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
)

func (d *DartClient) fetchURL(ctx context.Context, u string) ([]byte, error) {
	body, err := d.fetchURLHTTP(ctx, u)
	if err == nil {
		return body, nil
	}
	if !isDartTLSFailure(err) || runtime.GOOS != "windows" {
		return nil, err
	}
	return fetchURLViaCurl(ctx, u)
}

func (d *DartClient) fetchURLHTTP(ctx context.Context, u string) ([]byte, error) {
	if d.budget != nil {
		if err := d.budget.Check(ctx, apibudget.ProviderDART); err != nil {
			return nil, err
		}
	}
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
	if d.budget != nil {
		d.budget.Record(ctx, apibudget.ProviderDART)
	}
	return body, nil
}

func fetchURLViaCurl(ctx context.Context, u string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "curl.exe", "-sS", "-f", "-L", u)
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
