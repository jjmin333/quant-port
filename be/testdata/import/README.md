# Holdings screenshot OCR fixtures

Regression tests for the screenshot import pipeline (`POST /api/v1/import/holdings/screenshots`).

## Primary fixture (user-provided)

| File | Description |
|------|-------------|
| `domestic_holdings_samsung.png` | Dark-theme **국내주식** list (7 rows). Toss-style layout: logo + name + qty on the left, valuation + PnL on the right. |
| `domestic_etf_kiwoom.png` | Light-theme **키움자산** domestic ETF list (KODEX + TIGER 미국나스닥100, 785주 + 100주). |
| `domestic_kiwoom_samsung.png` | Light-theme **키움자산** domestic single holding (삼성전자 109주). |
| `overseas_watchlist_quotes.png` | US **watchlist/quote** screen (names + $ prices, no qty). |
| `overseas_holdings.png` | Dark-theme **해외주식** list (5 rows). USD valuation + PnL; OCR misreads names/qty across full-frame vs left-column passes. |

**Expected parse result (domestic):**

| 종목 | 수량 |
|------|------|
| 삼성SDI | 4주 |
| 삼성전자 | 34주 |
| 한화솔루션 | 55주 |
| 한화시스템 | 23주 |
| 현대차 | 4주 |
| LG전자 | 7주 |
| NAVER | 19주 |

**Expected parse result (Kiwoom domestic single stock):**

| 종목 | 수량 | 평단가 |
|------|------|--------|
| 삼성전자 | 109주 | 359,260원 |

**Expected parse result (Kiwoom domestic ETF):**

| 종목 | 수량 |
|------|------|
| KODEX 미국나스닥100 | 785주 |
| TIGER 미국나스닥100 | 100주 |

**Expected parse result (overseas watchlist — no holdings qty):**

| 종목 | 수량 | 평단가 |
|------|------|--------|
| (네vi우스 그룹 등) | *(비움)* | *(비움)* |

**Expected parse result (overseas):**

| 종목 | 수량 |
|------|------|
| 어플라이드 머티리얼즈 | 4주 |
| 아이렌 | 55주 |
| ARM 홀딩스(ADR) | 50주 |
| 앱러빈 (OCR: 맵러빈) | 15주 |
| 오라클 | 40주 |

Run integration test (requires Tesseract):

```bash
docker run --rm -v "$PWD/be:/src" -w /src golang:1.25-bookworm \
  bash -c 'apt-get update -qq && apt-get install -y -qq tesseract-ocr tesseract-ocr-kor tesseract-ocr-eng >/dev/null && \
  go test ./internal/import/pipeline/... -run TestParseHoldingsImageSamsung -v'
```

Overseas fixture:

```bash
go test ./internal/import/pipeline/... -run TestParseHoldingsImageOverseas -v
go run ./cmd/ocr-debug ./testdata/import/overseas_holdings.png
```

Debug a single image:

```bash
go run ./cmd/ocr-debug ./testdata/import/domestic_holdings_samsung.png
```

## Additional text-only samples (unit tests)

These live in `internal/import/textparse/*_test.go` and document OCR output shapes we handle:

| Sample | File | Notes |
|--------|------|-------|
| Domestic qty + finance columns | `parser_test.go` `TestParseHoldingsTextDomesticWithFinance` | Name / N주 / evaluation / PnL |
| Overseas USD list | `parser_test.go` `TestParseHoldingsTextOverseasScreenshot` | `$` amounts, ADR names, `@` qty hint |
| Raw Tesseract (Samsung screenshot) | `parser_samsung_test.go` | Exact stdout from Tesseract PSM 11 |
| OCR name typos | `ocr_fix_test.go` | `삼성5미`→삼성SDI, `16전자`→LG전자 |
| Fuzzy resolver typos | `resolver/fuzzy_test.go` | Catalog match for OCR misreads |

## Tips for new fixtures

1. Save PNG/JPG under `be/testdata/import/`.
2. Add a pipeline test mirroring `ocr_passes_test.go`.
3. If Tesseract reads new garbage tokens, add a row to `ocr_fix.go` or `parser.go` filters — prefer **majority merge** over single-pass heuristics.
