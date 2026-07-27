package seed

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/transform"
)

const (
	krxColISIN       = 0
	krxColSymbol     = 1
	krxColNameFull   = 2
	krxColNameShort  = 3
	krxColEnglish    = 4
	krxColListDate   = 5
	krxColMarket     = 6
	krxColSecType    = 7
	krxColShares     = 11
)

type USListing struct {
	Symbol  string `json:"symbol"`
	Name    string `json:"name"`
	IPOYear string `json:"ipoyear"`
}

func LoadKRXCSV(path string) ([]model.Stock, error) {
	header, err := peekKRXHeader(path, false)
	if err != nil {
		return nil, err
	}
	useEUC := !hasKRXHeader(header)
	return loadKRXCSVWithEncoding(path, useEUC)
}

func peekKRXHeader(path string, eucKR bool) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var reader io.Reader = f
	if eucKR {
		reader = transform.NewReader(f, korean.EUCKR.NewDecoder())
	}
	r := csv.NewReader(reader)
	r.LazyQuotes = true
	header, err := r.Read()
	if err != nil {
		return nil, err
	}
	if len(header) > 0 {
		header[0] = strings.TrimPrefix(header[0], "\ufeff")
	}
	return header, nil
}

func hasKRXHeader(header []string) bool {
	_, ok := indexColumns(header)["단축코드"]
	return ok
}

func loadKRXCSVWithEncoding(path string, eucKR bool) ([]model.Stock, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var reader io.Reader = f
	if eucKR {
		reader = transform.NewReader(f, korean.EUCKR.NewDecoder())
	}

	r := csv.NewReader(reader)
	r.LazyQuotes = true
	r.FieldsPerRecord = -1

	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}
	if len(header) > 0 {
		header[0] = strings.TrimPrefix(header[0], "\ufeff")
	}
	col := indexColumns(header)

	var out []model.Stock
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read row: %w", err)
		}
		stock, ok := mapKRXRow(row, col)
		if !ok {
			continue
		}
		out = append(out, stock)
	}
	return out, nil
}

func LoadUSJSON(path, market string) ([]model.Stock, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var listings []USListing
	if err := json.NewDecoder(f).Decode(&listings); err != nil {
		return nil, fmt.Errorf("decode json: %w", err)
	}

	out := make([]model.Stock, 0, len(listings))
	for _, item := range listings {
		symbol := strings.ToUpper(strings.TrimSpace(item.Symbol))
		name := strings.TrimSpace(item.Name)
		if symbol == "" || name == "" {
			continue
		}
		assetType := model.AssetUS
		upperName := strings.ToUpper(name)
		if strings.Contains(upperName, " ETF") || strings.HasSuffix(upperName, " ETF") {
			assetType = model.AssetETF
		}
		out = append(out, model.Stock{
			Symbol:       symbol,
			Name:         trimName(name, 200),
			EnglishName:  trimName(name, 200),
			Market:       market,
			SecurityType: "EQUITY",
			Currency:     model.CurrencyUSD,
			AssetType:    assetType,
			Status:       "NORMAL",
			ListDate:     parseUSIPOYear(item.IPOYear),
		})
	}
	return out, nil
}

func indexColumns(header []string) map[string]int {
	idx := make(map[string]int, len(header))
	for i, h := range header {
		idx[strings.TrimSpace(h)] = i
	}
	return idx
}

func mapKRXRow(row []string, col map[string]int) (model.Stock, bool) {
	get := func(key string, idx int) string {
		if i, ok := col[key]; ok && i < len(row) {
			return strings.TrimSpace(row[i])
		}
		if idx >= 0 && idx < len(row) {
			return strings.TrimSpace(row[idx])
		}
		return ""
	}

	symbol := get("단축코드", krxColSymbol)
	name := get("한글 종목약명", krxColNameShort)
	if name == "" {
		name = get("한글 종목명", krxColNameFull)
	}
	if symbol == "" || name == "" {
		return model.Stock{}, false
	}

	marketRaw := get("시장구분", krxColMarket)
	market := mapKRXMarket(marketRaw)
	if market == "" {
		return model.Stock{}, false
	}

	listDate, _ := parseKRXDate(get("상장일", krxColListDate))
	shares := parseShares(get("상장주식수", krxColShares))

	assetType := model.AssetDomestic
	secType := get("증권구분", krxColSecType)
	if strings.Contains(secType, "ETF") {
		assetType = model.AssetETF
	}

	return model.Stock{
		Symbol:            symbol,
		Name:              trimName(name, 200),
		EnglishName:       trimName(get("영문 종목명", krxColEnglish), 200),
		ISINCode:          get("표준코드", krxColISIN),
		Market:            market,
		SecurityType:      secType,
		Currency:          model.CurrencyKRW,
		AssetType:         assetType,
		Status:            "NORMAL",
		SharesOutstanding: shares,
		ListDate:          listDate,
	}, true
}

func mapKRXMarket(raw string) string {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "KOSPI":
		return "KRX"
	case "KOSDAQ", "KOSDAQ GLOBAL":
		return "KOSDAQ"
	case "KONEX":
		return "KONEX"
	default:
		return ""
	}
}

func parseUSIPOYear(raw string) *time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	year, err := strconv.Atoi(raw)
	if err != nil || year < 1800 || year > 2100 {
		return nil
	}
	t := time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC)
	return &t
}

func parseKRXDate(raw string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	for _, layout := range []string{"2006/01/02", "2006-01-02"} {
		if t, err := time.Parse(layout, raw); err == nil {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("invalid date: %s", raw)
}

func parseShares(raw string) *int64 {
	raw = strings.ReplaceAll(strings.TrimSpace(raw), ",", "")
	if raw == "" {
		return nil
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n <= 0 {
		return nil
	}
	return &n
}

func trimName(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
