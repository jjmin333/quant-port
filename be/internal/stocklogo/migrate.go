package stocklogo

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

// MigrateOptions configures a one-time logo migration run.
type MigrateOptions struct {
	OutDir       string
	Stocks       []model.Stock
	NaverDelay   time.Duration
	FMPWorkers   int
	HTTPClient   *http.Client
	SkipDomestic bool
	DeltaOnly    bool
	Existing     *Manifest
}

type usResult struct {
	symbol string
	entry  ManifestEntry
}

// RunMigrate downloads domestic logos from Naver CDN (via mobile JSON) and probes FMP for US rows.
func RunMigrate(ctx context.Context, opt MigrateOptions) (*Manifest, error) {
	if opt.OutDir == "" {
		return nil, fmt.Errorf("out dir required")
	}
	if opt.NaverDelay <= 0 {
		opt.NaverDelay = 120 * time.Millisecond
	}
	if opt.FMPWorkers <= 0 {
		opt.FMPWorkers = 12
	}
	client := opt.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}

	krDir := filepath.Join(opt.OutDir, "kr")
	if err := os.MkdirAll(krDir, 0o755); err != nil {
		return nil, err
	}

	m := &Manifest{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Entries:     map[string]ManifestEntry{},
	}
	if opt.Existing != nil {
		m.GeneratedAt = opt.Existing.GeneratedAt
		if m.GeneratedAt == "" {
			m.GeneratedAt = time.Now().UTC().Format(time.RFC3339)
		}
		m.Summary = opt.Existing.Summary
		for sym, entry := range opt.Existing.Entries {
			m.Entries[sym] = entry
		}
	}

	stocks := FilterDeltaLogoStocks(opt.Stocks, m, krDir, opt.DeltaOnly)

	var usStocks []model.Stock
	for _, s := range stocks {
		sym := strings.ToUpper(strings.TrimSpace(s.Symbol))
		if sym == "" {
			continue
		}
		if IsDomestic(s.Currency, s.AssetType) {
			if opt.SkipDomestic {
				continue
			}
			processDomestic(ctx, client, krDir, m, sym, opt.NaverDelay)
			continue
		}
		usStocks = append(usStocks, s)
	}

	if len(usStocks) == 0 {
		return m, nil
	}

	if opt.SkipDomestic {
		domestic := m.Summary.Domestic
		m.Summary.US = struct {
			Total          int `json:"total"`
			NaverSkipped   int `json:"naverSkipped"`
			FMPAvailable   int `json:"fmpAvailable"`
			FMPUnavailable int `json:"fmpUnavailable"`
		}{}
		m.Summary.Domestic = domestic
		for sym, entry := range m.Entries {
			if entry.Region == "us" {
				delete(m.Entries, sym)
			}
		}
	}

	m.Summary.US.Total = len(usStocks)
	m.Summary.US.NaverSkipped = len(usStocks)

	jobs := make(chan model.Stock, len(usStocks))
	results := make(chan usResult, len(usStocks))
	workers := opt.FMPWorkers
	if workers > len(usStocks) {
		workers = len(usStocks)
	}

	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for s := range jobs {
				sym := strings.ToUpper(strings.TrimSpace(s.Symbol))
				entry := ManifestEntry{Region: "us", Source: "fmp"}
				if !ProbeFMPImage(ctx, client, sym, s.Currency, s.AssetType) {
					entry.Reason = "fmp 404"
				}
				results <- usResult{symbol: sym, entry: entry}
			}
		}()
	}

	for _, s := range usStocks {
		jobs <- s
	}
	close(jobs)
	wg.Wait()
	close(results)

	for r := range results {
		if r.entry.Reason != "" {
			m.Summary.US.FMPUnavailable++
		} else {
			m.Summary.US.FMPAvailable++
		}
		m.Entries[r.symbol] = r.entry
	}

	return m, nil
}

func processDomestic(ctx context.Context, client *http.Client, krDir string, m *Manifest, sym string, delay time.Duration) {
	m.Summary.Domestic.Total++
	entry := ManifestEntry{Region: "domestic"}

	logoURL, err := FetchNaverLogoPNGURL(ctx, client, sym)
	if err != nil {
		entry.Source = "fmp"
		entry.Reason = err.Error()
		m.Summary.Domestic.NaverFail++
		m.Summary.Domestic.FMPFallback++
		m.Entries[sym] = entry
		time.Sleep(delay)
		return
	}

	dest := filepath.Join(krDir, sym+".png")
	if err := downloadFile(ctx, client, logoURL, dest); err != nil {
		entry.Source = "fmp"
		entry.Reason = err.Error()
		m.Summary.Domestic.NaverFail++
		m.Summary.Domestic.FMPFallback++
		m.Entries[sym] = entry
		time.Sleep(delay)
		return
	}

	entry.Source = "naver"
	entry.Path = localKRPath(sym)
	m.Summary.Domestic.NaverSuccess++
	m.Entries[sym] = entry
	time.Sleep(delay)
}

func downloadFile(ctx context.Context, client *http.Client, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "quant-port-migrate-logos/1.0")

	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: HTTP %d", url, res.StatusCode)
	}

	tmp := dest + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, res.Body); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dest)
}
