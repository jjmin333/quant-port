package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/enact/quant-port/be/internal/authctx"
	"github.com/enact/quant-port/be/internal/import/costestimate"
	"github.com/enact/quant-port/be/internal/import/imageutil"
	"github.com/enact/quant-port/be/internal/import/ocr"
	"github.com/enact/quant-port/be/internal/import/resolver"
	"github.com/enact/quant-port/be/internal/import/pipeline"
	"github.com/enact/quant-port/be/internal/import/textparse"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/safego"
	"github.com/redis/go-redis/v9"
)

const (
	maxImportImages   = 10
	importJobCacheTTL = 24 * time.Hour
)

type ImportJobService struct {
	redis       *redis.Client
	holdings    *UserHoldingsService
	watchlist   *WatchlistService
	resolver    *resolver.Resolver
	ocr         ocr.Engine
	tempRoot    string

	jobsWg sync.WaitGroup
}

func NewImportJobService(
	redisClient *redis.Client,
	holdings *UserHoldingsService,
	watchlist *WatchlistService,
	res *resolver.Resolver,
	tempRoot string,
) *ImportJobService {
	if strings.TrimSpace(tempRoot) == "" {
		tempRoot = filepath.Join("tmp", "import")
	}
	tempRoot = filepath.Clean(tempRoot)
	if err := os.MkdirAll(tempRoot, 0o755); err != nil {
		log.Printf("import: temp dir %s: %v (job uploads may fail)", tempRoot, err)
	}

	engine := ocr.DefaultEngine()
	if engine.Available() {
		log.Printf("import: tesseract ready (langs=%s, temp=%s)", engine.Langs, tempRoot)
	} else {
		log.Printf("import: tesseract not found — OCR disabled; manual symbol entry still works (temp=%s)", tempRoot)
	}

	svc := &ImportJobService{
		redis:     redisClient,
		holdings:  holdings,
		watchlist: watchlist,
		resolver:  res,
		ocr:       engine,
		tempRoot:  tempRoot,
	}
	safego.Go("import-orphan-sweeper", svc.runOrphanSweeper)
	return svc
}

func importJobCacheKey(jobID string) string {
	return fmt.Sprintf("import:holdings:job:%s", jobID)
}

func (s *ImportJobService) StartScreenshotsImport(
	ctx context.Context,
	files []*multipart.FileHeader,
	meta []model.ImportImageMeta,
) (string, error) {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		return "", fmt.Errorf("at least one image required")
	}
	if len(files) > maxImportImages {
		return "", fmt.Errorf("maximum %d images allowed", maxImportImages)
	}
	if len(meta) > 0 && len(meta) != len(files) {
		return "", fmt.Errorf("meta count must match image count")
	}

	jobID, err := newJobID()
	if err != nil {
		return "", err
	}
	jobDir := filepath.Join(s.tempRoot, jobID)
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		return "", err
	}

	saved := make([]string, 0, len(files))
	for i, fh := range files {
		ext := strings.ToLower(filepath.Ext(fh.Filename))
		if ext == "" {
			ext = ".jpg"
		}
		if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
			_ = os.RemoveAll(jobDir)
			return "", fmt.Errorf("JPG 또는 PNG 사진만 올릴 수 있습니다 (받은 형식: %s)", ext)
		}
		dst := filepath.Join(jobDir, fmt.Sprintf("img_%02d%s", i, ext))
		if err := saveUpload(fh, dst); err != nil {
			_ = os.RemoveAll(jobDir)
			return "", err
		}
		saved = append(saved, dst)
	}

	now := time.Now()
	snap := model.ImportJobSnapshot{
		JobID:     jobID,
		UserID:    userID,
		Status:    model.ImportJobProcessing,
		Progress:  model.ImportJobProgress{Done: 0, Total: len(saved)},
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.saveJob(ctx, snap); err != nil {
		_ = os.RemoveAll(jobDir)
		return "", err
	}

	safego.Go("import-holdings-job", func() {
		s.processJob(jobID, userID, saved, meta)
	})

	return jobID, nil
}

// WaitIdle blocks until every in-flight screenshot import job finishes or ctx
// is cancelled. Called during API shutdown so OCR workers do not outlive pools.
func (s *ImportJobService) WaitIdle(ctx context.Context) {
	done := make(chan struct{})
	go func() {
		s.jobsWg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
	}
}

func saveUpload(fh *multipart.FileHeader, dst string) error {
	src, err := fh.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, src)
	return err
}

func (s *ImportJobService) GetJob(ctx context.Context, jobID string) (*model.ImportJobSnapshot, error) {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return nil, err
	}
	snap, err := s.loadJob(ctx, jobID)
	if err != nil {
		return nil, err
	}
	if snap.UserID != userID {
		return nil, fmt.Errorf("job not found")
	}
	return snap, nil
}

func (s *ImportJobService) CommitJob(ctx context.Context, jobID string, req model.ImportCommitRequest) (int, int, error) {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return 0, 0, err
	}
	snap, err := s.loadJob(ctx, jobID)
	if err != nil {
		return 0, 0, err
	}
	if snap.UserID != userID {
		return 0, 0, fmt.Errorf("job not found")
	}
	if snap.Status != model.ImportJobPreview && snap.Status != model.ImportJobFailed {
		return 0, 0, fmt.Errorf("job not ready for commit")
	}

	count, err := s.holdings.CommitImportRows(ctx, req)
	if err != nil {
		return 0, 0, err
	}

	watchAdded := 0
	if req.AddToWatchlist {
		for _, row := range req.Rows {
			if !row.Selected {
				continue
			}
			sym := strings.TrimSpace(strings.ToUpper(row.Symbol))
			if sym == "" {
				continue
			}
			if err := s.watchlist.Add(ctx, sym); err == nil {
				watchAdded++
			}
		}
	}

	snap.Status = model.ImportJobCommitted
	snap.UpdatedAt = time.Now()
	_ = s.saveJob(ctx, *snap)
	_ = os.RemoveAll(filepath.Join(s.tempRoot, jobID))
	return count, watchAdded, nil
}

func (s *ImportJobService) processJob(jobID string, userID int64, paths []string, meta []model.ImportImageMeta) {
	s.jobsWg.Add(1)
	defer s.jobsWg.Done()

	ctx := authctx.WithUserID(context.Background(), userID)
	defer func() {
		if rec := recover(); rec != nil {
			log.Printf("import job %s panic: %v\n%s", jobID, rec, debug.Stack())
			s.markJobFailed(ctx, jobID, len(paths), "보유 종목을 불러오지 못했습니다. 직접 입력하시거나 스크린샷을 다시 첨부해주세요.")
		}
	}()

	total := len(paths)
	var done atomic.Int32

	var mu sync.Mutex
	var parsed []textparse.ParsedLine
	var allOCRTexts []string
	var brokerHints []string
	imageResults := make([]model.ImportImageResult, total)

	var wg sync.WaitGroup
	for i, path := range paths {
		wg.Add(1)
		go func(idx int, srcPath string) {
			defer wg.Done()

			fail := func(reason string) {
				mu.Lock()
				imageResults[idx] = model.ImportImageResult{Index: idx, Success: false, Error: reason}
				mu.Unlock()
			}

			defer func() {
				done.Add(1)
				s.updateProgress(ctx, jobID, int(done.Load()), total)
			}()
			defer func() {
				if rec := recover(); rec != nil {
					log.Printf("import job %s image %d panic: %v\n%s", jobID, idx, rec, debug.Stack())
					fail("이미지를 처리하는 중 오류가 발생했습니다")
				}
			}()

			var crop *model.ImportCropRect
			brokerHint := ""
			if idx < len(meta) {
				crop = meta[idx].Crop
				brokerHint = meta[idx].BrokerHint
			}

			jobDir := filepath.Join(s.tempRoot, jobID)
			cropped := filepath.Join(jobDir, fmt.Sprintf("crop_%02d.png", idx))
			if err := imageutil.CropImageFile(srcPath, cropped, crop); err != nil {
				log.Printf("import job %s crop %d: %v", jobID, idx, err)
				fail("이미지를 읽을 수 없습니다")
				return
			}
			preprocessed := filepath.Join(jobDir, fmt.Sprintf("ocr_%02d.png", idx))
			scaled := filepath.Join(jobDir, fmt.Sprintf("ocr_%02d_x2.png", idx))
			_ = imageutil.UpscaleFile(cropped, scaled, 2)
			if err := imageutil.PreprocessForOCR(scaled, preprocessed); err != nil {
				preprocessed = scaled
			}

			ocrWork := filepath.Join(jobDir, fmt.Sprintf("ocrwork_%02d", idx))
			parseResult := pipeline.ParseHoldingsImage(s.ocr, cropped, preprocessed, ocrWork)
			if msg := textparse.RejectNonBrokerageScreenshot(parseResult.OCRTexts, parseResult.Lines); msg != "" {
				fail(msg)
				return
			}
			if len(parseResult.Lines) == 0 {
				fail("보유 종목을 찾지 못했습니다")
				return
			}

			mu.Lock()
			imageResults[idx] = model.ImportImageResult{Index: idx, Success: true}
			parsed = append(parsed, parseResult.Lines...)
			allOCRTexts = append(allOCRTexts, parseResult.OCRTexts...)
			if brokerHint != "" {
				brokerHints = append(brokerHints, brokerHint)
			}
			mu.Unlock()
		}(i, path)
	}
	wg.Wait()

	merged := textparse.MergeParsedLines(parsed)
	merged = textparse.FilterLinesForLocale(merged, textparse.InferHoldingsLocale(allOCRTexts, merged))
	merged = textparse.FilterParsedLines(merged)
	preview := make([]model.ImportPreviewRow, 0, len(merged))
	for i, line := range merged {
		displayName := textparse.NormalizeDisplayName(line.Name)
		res := s.resolver.ResolveName(ctx, displayName)
		row := model.ImportPreviewRow{
			ID:             fmt.Sprintf("row-%d", i+1),
			RawName:        line.Name,
			Quantity:       line.Quantity,
			Selected:       true,
			SourceImageIdx: 0,
		}
		if res.Error != "" {
			row.NeedsReview = true
			row.ResolveError = res.Error
			row.Confidence = 0.2
		} else {
			row.Symbol = res.Symbol
			row.Name = res.Name
			row.RawName = res.Name
			row.Confidence = res.Confidence
			row.NeedsReview = res.Confidence < 0.85
		}
		if textparse.LooksLikeOCRFragment(displayName) {
			continue
		}
		if line.Quantity <= 0 {
			row.NeedsReview = true
			row.Quantity = 0
		} else if line.Quantity > 500 {
			row.NeedsReview = true
		}
		if line.Quantity > 0 {
			cost := costestimate.Estimate(line)
			if cost.AvgPrice != nil {
				row.AvgPrice = cost.AvgPrice
				row.CostCurrency = cost.Currency
			}
		}
		if len(brokerHints) > 0 {
			row.BrokerHint = brokerHints[0]
		}
		preview = append(preview, row)
	}
	preview = dedupePreviewBySymbol(preview)
	preview = dedupeResolvedPhantomRows(preview)

	previewInputs := make([]textparse.PreviewRowInput, len(preview))
	for i, row := range preview {
		previewInputs[i] = textparse.PreviewRowInput{
			RawName:    row.RawName,
			Symbol:     row.Symbol,
			Confidence: row.Confidence,
			Quantity:   row.Quantity,
		}
	}
	if len(preview) > 0 {
		if msg := textparse.RejectPreviewAfterResolve(allOCRTexts, merged, previewInputs); msg != "" {
			snap, err := s.loadJob(ctx, jobID)
			if err != nil {
				return
			}
			snap.Progress = model.ImportJobProgress{Done: total, Total: total}
			snap.Preview = nil
			snap.ImageResults = imageResults
			snap.Status = model.ImportJobFailed
			snap.Error = msg
			snap.UpdatedAt = time.Now()
			_ = s.saveJob(ctx, *snap)
			_ = os.RemoveAll(filepath.Join(s.tempRoot, jobID))
			return
		}
	}

	snap, err := s.loadJob(ctx, jobID)
	if err != nil {
		return
	}
	snap.Progress = model.ImportJobProgress{Done: total, Total: total}
	snap.Preview = preview
	snap.ImageResults = imageResults
	snap.UpdatedAt = time.Now()

	failedNums := failedImageNumbers(imageResults)
	if len(preview) > 0 && len(failedNums) > 0 {
		snap.Status = model.ImportJobPreview
		snap.Warning = formatPartialImportWarning(failedNums)
		snap.Error = ""
	} else if len(preview) > 0 {
		snap.Status = model.ImportJobPreview
		snap.Warning = ""
		snap.Error = ""
	} else {
		snap.Status = model.ImportJobFailed
		snap.Warning = ""
		if !s.ocr.Available() {
			snap.Error = "보유 종목을 불러오지 못했습니다. 직접 입력하시거나 스크린샷을 다시 첨부해주세요."
		} else if len(failedNums) > 0 {
			snap.Error = formatTotalImportFailureMessage(imageResults)
		} else {
			snap.Error = formatTotalImportFailureMessage(imageResults)
		}
	}
	_ = s.saveJob(ctx, *snap)
	_ = os.RemoveAll(filepath.Join(s.tempRoot, jobID))
}

func failedImageNumbers(results []model.ImportImageResult) []int {
	var out []int
	for _, r := range results {
		if !r.Success {
			out = append(out, r.Index+1) // 1-based for user display
		}
	}
	return out
}

func formatPartialImportWarning(failedNums []int) string {
	return fmt.Sprintf("%s번째 화면을 읽지 못했습니다. 직접 입력하시겠습니까, 또는 스크린샷을 다시 찍어 주세요.",
		joinImageNumbers(failedNums))
}

func formatTotalImportFailureMessage(results []model.ImportImageResult) string {
	for _, r := range results {
		if msg := strings.TrimSpace(r.Error); msg != "" {
			return msg
		}
	}
	return "보유 종목을 불러오지 못했습니다. 직접 입력하시거나 스크린샷을 다시 첨부해주세요."
}

func joinImageNumbers(nums []int) string {
	parts := make([]string, len(nums))
	for i, n := range nums {
		parts[i] = fmt.Sprintf("%d", n)
	}
	return strings.Join(parts, ", ")
}

func (s *ImportJobService) recognize(imagePath string) (string, error) {
	if s.ocr.Available() {
		return s.ocr.RecognizeFile(imagePath)
	}
	return "", fmt.Errorf("tesseract unavailable")
}

// resolvedPhantomPairs drops false-positive symbols when the canonical ticker is already present at the same qty.
var resolvedPhantomPairs = map[string]string{
	"065660": "035420", // 안트로젠 debris when NAVER (19주) already parsed
}

func dedupeResolvedPhantomRows(rows []model.ImportPreviewRow) []model.ImportPreviewRow {
	if len(rows) <= 1 {
		return rows
	}
	byQty := map[float64]map[string]struct{}{}
	for _, row := range rows {
		sym := strings.ToUpper(strings.TrimSpace(row.Symbol))
		if sym == "" || row.Quantity <= 0 {
			continue
		}
		if byQty[row.Quantity] == nil {
			byQty[row.Quantity] = map[string]struct{}{}
		}
		byQty[row.Quantity][sym] = struct{}{}
	}
	drop := map[string]struct{}{}
	for _, syms := range byQty {
		for phantom, canonical := range resolvedPhantomPairs {
			if _, hasPhantom := syms[phantom]; !hasPhantom {
				continue
			}
			if _, hasCanonical := syms[canonical]; hasCanonical {
				drop[phantom] = struct{}{}
			}
		}
	}
	if len(drop) == 0 {
		return rows
	}
	out := make([]model.ImportPreviewRow, 0, len(rows))
	for _, row := range rows {
		if _, skip := drop[strings.ToUpper(strings.TrimSpace(row.Symbol))]; skip {
			continue
		}
		out = append(out, row)
	}
	for i := range out {
		out[i].ID = fmt.Sprintf("row-%d", i+1)
	}
	return out
}

// dedupePreviewBySymbol merges rows that resolved to the same ticker (dual OCR name variants).
func dedupePreviewBySymbol(rows []model.ImportPreviewRow) []model.ImportPreviewRow {
	if len(rows) <= 1 {
		return rows
	}
	bySymbol := map[string]model.ImportPreviewRow{}
	order := make([]string, 0)
	var unresolved []model.ImportPreviewRow

	for _, row := range rows {
		sym := strings.ToUpper(strings.TrimSpace(row.Symbol))
		if sym == "" {
			unresolved = append(unresolved, row)
			continue
		}
		if existing, ok := bySymbol[sym]; ok {
			oldQty := existing.Quantity
			newQty := row.Quantity
			existing.Quantity = mergePreviewQty(oldQty, newQty)
			if mergedAvg := mergePreviewAvgPrice(oldQty, existing.AvgPrice, newQty, row.AvgPrice); mergedAvg != nil {
				existing.AvgPrice = mergedAvg
				if row.CostCurrency != "" && existing.CostCurrency == "" {
					existing.CostCurrency = row.CostCurrency
				}
			} else if row.AvgPrice != nil && existing.AvgPrice == nil {
				existing.AvgPrice = row.AvgPrice
				existing.CostCurrency = row.CostCurrency
			}
			if row.Confidence > existing.Confidence || existing.Name == "" {
				existing.RawName = row.RawName
				existing.Name = row.Name
				existing.Confidence = row.Confidence
			}
			existing.NeedsReview = existing.NeedsReview || row.NeedsReview
			bySymbol[sym] = existing
			continue
		}
		bySymbol[sym] = row
		order = append(order, sym)
	}

	out := make([]model.ImportPreviewRow, 0, len(order)+len(unresolved))
	for _, sym := range order {
		out = append(out, bySymbol[sym])
	}
	out = append(out, unresolved...)
	for i := range out {
		out[i].ID = fmt.Sprintf("row-%d", i+1)
	}
	return out
}

func mergePreviewQty(existing, next float64) float64 {
	if existing <= 0 {
		return next
	}
	if next <= 0 {
		return existing
	}
	if existing == next {
		return existing
	}
	return existing + next
}

func mergePreviewAvgPrice(existingQty float64, existingAvg *float64, newQty float64, newAvg *float64) *float64 {
	if existingAvg == nil && newAvg == nil {
		return nil
	}
	if existingAvg == nil {
		return newAvg
	}
	if newAvg == nil {
		return existingAvg
	}
	totalQty := existingQty + newQty
	if totalQty <= 0 {
		return existingAvg
	}
	merged := (*existingAvg*existingQty + *newAvg*newQty) / totalQty
	return &merged
}

func (s *ImportJobService) updateProgress(ctx context.Context, jobID string, done, total int) {
	snap, err := s.loadJob(ctx, jobID)
	if err != nil {
		return
	}
	snap.Progress = model.ImportJobProgress{Done: done, Total: total}
	snap.UpdatedAt = time.Now()
	_ = s.saveJob(ctx, *snap)
}

func (s *ImportJobService) saveJob(ctx context.Context, snap model.ImportJobSnapshot) error {
	if s.redis == nil {
		return fmt.Errorf("redis unavailable")
	}
	payload, err := json.Marshal(snap)
	if err != nil {
		return err
	}
	return s.redis.Set(ctx, importJobCacheKey(snap.JobID), payload, importJobCacheTTL).Err()
}

func (s *ImportJobService) loadJob(ctx context.Context, jobID string) (*model.ImportJobSnapshot, error) {
	if s.redis == nil {
		return nil, fmt.Errorf("redis unavailable")
	}
	raw, err := s.redis.Get(ctx, importJobCacheKey(jobID)).Bytes()
	if err != nil {
		return nil, fmt.Errorf("job not found")
	}
	var snap model.ImportJobSnapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		return nil, err
	}
	return &snap, nil
}

func newJobID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *ImportJobService) markJobFailed(ctx context.Context, jobID string, total int, message string) {
	snap, err := s.loadJob(ctx, jobID)
	if err != nil {
		return
	}
	snap.Status = model.ImportJobFailed
	snap.Error = message
	snap.Progress = model.ImportJobProgress{Done: total, Total: total}
	snap.UpdatedAt = time.Now()
	_ = s.saveJob(ctx, *snap)
	_ = os.RemoveAll(filepath.Join(s.tempRoot, jobID))
}

func (s *ImportJobService) runOrphanSweeper() {
	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		entries, err := os.ReadDir(s.tempRoot)
		if err != nil {
			continue
		}
		cutoff := time.Now().Add(-24 * time.Hour)
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			if info.ModTime().Before(cutoff) {
				_ = os.RemoveAll(filepath.Join(s.tempRoot, e.Name()))
			}
		}
	}
}
