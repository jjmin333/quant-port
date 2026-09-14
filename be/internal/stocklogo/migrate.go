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
	// StoreUS downloads overseas logos into `us/*.png` instead of only probing
	// the FMP CDN for a 200. See processUS.
	StoreUS bool
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
	usDir := filepath.Join(opt.OutDir, "us")
	if opt.StoreUS {
		if err := os.MkdirAll(usDir, 0o755); err != nil {
			return nil, err
		}
	}

	m := &Manifest{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		// 매 실행이 갱신한다. GeneratedAt과 달리 델타 실행도 여기는 새로 쓴다 —
		// 월간 갱신이 언제 돌았는지가 그 작업의 유일한 상태값이다.
		RefreshedAt: time.Now().UTC().Format(time.RFC3339),
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

	stocks := FilterDeltaLogoStocks(opt.Stocks, m, krDir, usDir, opt.DeltaOnly, opt.StoreUS)

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
				results <- usResult{symbol: sym, entry: processUS(ctx, client, usDir, sym, s, opt.StoreUS)}
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

/*
processUS resolves one overseas symbol's logo.

두 가지 모드가 있다.

  - `store=false` (기본): FMP CDN에 그림이 있는지 **HEAD로 확인만** 한다. 화면은
    런타임에 FMP URL을 직접 부른다. 저장소가 가벼운 대신 이미지 하나하나가 외부
    CDN에 매달린다.
  - `store=true`: 실제로 내려받아 `us/{SYM}.png`로 저장하고 매니페스트에 경로를
    적는다. 국내가 이미 그렇게 하고 있고(`kr/*.png`), 그림이 우리 것이 되면 FMP가
    URL 규칙을 바꾸거나 그 종목을 내려도 화면이 그대로다.

받아 두더라도 실패한 종목은 `source: "fmp"`에 경로 없이 남는다 — 화면이 그때는
예전처럼 원격 URL로 돌아간다(`fe/src/lib/stock-logo.ts`).
*/
func processUS(
	ctx context.Context, client *http.Client, usDir, sym string, s model.Stock, store bool,
) ManifestEntry {
	entry := ManifestEntry{Region: "us", Source: "fmp"}
	url := FMPImageURL(sym, s.Currency, s.AssetType)
	if url == "" {
		entry.Reason = "no fmp key"
		return entry
	}
	if !store {
		if !ProbeFMPImage(ctx, client, sym, s.Currency, s.AssetType) {
			entry.Reason = "fmp 404"
		}
		return entry
	}

	// 심볼에는 `BRK-B`처럼 파일명에 쓸 수 없는 글자가 섞이지 않지만, 경로 조작만은
	// 막는다 — 매니페스트 키가 그대로 파일 경로가 되는 자리다.
	if strings.ContainsAny(sym, `/\.`) {
		entry.Reason = "unsafe symbol"
		return entry
	}
	dest := filepath.Join(usDir, sym+".png")
	if err := downloadFile(ctx, client, url, dest); err != nil {
		entry.Reason = err.Error()
		return entry
	}
	entry.Path = localUSPath(sym)
	// 흰 로고를 흰 카드에 얹지 않으려면 받는 자리에서 한 번 재 둬야 한다 — tone.go 참고.
	entry.Tone = LogoTone(dest)
	return entry
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
	entry.Tone = LogoTone(dest)
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
