package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/enact/quant-port/be/internal/model"
)

// dartSweepPageSize — list.json이 허용하는 최대 페이지 크기.
// 크게 잡을수록 하루치를 적은 호출로 끝낸다(실측 397건/일 → 4페이지).
const dartSweepPageSize = 100

// dartSweepMaxPages bounds one day's sweep. 하루 접수량이 폭증해도 배치가
// 무한히 돌지 않게 하는 안전판이다 — 100페이지면 10,000건으로, 실측 일평균의
// 25배다.
const dartSweepMaxPages = 100

// DayDisclosure is one filing from the market-wide sweep.
type DayDisclosure struct {
	StockCode   string
	ReceiptNo   string
	ReceiptDate string // YYYY-MM-DD
	Title       string
	Filer       string
}

// FetchDayDisclosures returns every DART filing received on `day`.
//
// **corp_code 없이** 날짜만으로 부른다 — 이것이 이 함수의 존재 이유다. 종목별로
// 물으면 국내 상장 4,400종목에 4,400호출이 들지만, 시장 전체 스윕은 페이지 몇
// 번이면 끝난다. 조회 경로가 종목마다 DART를 부르던 구조를 이걸로 대체한다.
//
// gate는 **페이지마다** 잡았다 놓는다. 하루치 전체를 붙잡고 있으면 그동안 다른
// DART 배치가 통째로 멈추고, 호출자가 미리 잡은 채로 들어오면 Go 뮤텍스는
// 재진입이 안 되므로 그 자리에서 교착한다(실제로 그렇게 10분을 멈춰 봤다).
func (d *DartClient) FetchDayDisclosures(ctx context.Context, day time.Time, gate *providerGate) ([]DayDisclosure, int, error) {
	if !d.cfg.Enabled() {
		return nil, 0, fmt.Errorf("dart api key not configured")
	}
	dateKey := day.In(model.KST).Format("20060102")
	isoDate := day.In(model.KST).Format("2006-01-02")

	var out []DayDisclosure
	pages := 0
	for page := 1; page <= dartSweepMaxPages; page++ {
		if ctx.Err() != nil {
			return out, pages, ctx.Err()
		}
		u := fmt.Sprintf(
			"https://opendart.fss.or.kr/api/list.json?crtfc_key=%s&bgn_de=%s&end_de=%s&page_no=%d&page_count=%d",
			url.QueryEscape(d.cfg.APIKey), dateKey, dateKey, page, dartSweepPageSize,
		)
		release := gate.enter(ctx)
		body, err := d.fetchURL(ctx, u)
		release()
		if err != nil {
			return out, pages, err
		}
		pages++

		var payload struct {
			Status    string `json:"status"`
			Message   string `json:"message"`
			TotalPage int    `json:"total_page"`
			List      []struct {
				StockCode  string `json:"stock_code"`
				ReportName string `json:"report_nm"`
				ReceiptNo  string `json:"rcept_no"`
				FilerName  string `json:"flr_nm"`
			} `json:"list"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			return out, pages, err
		}
		// "013" = 조회된 데이터가 없습니다. 휴장일이면 정상적으로 비어 있다.
		if payload.Status == "013" {
			return out, pages, nil
		}
		if payload.Status != "000" && payload.Status != "" {
			return out, pages, fmt.Errorf("dart list sweep: %s", payload.Message)
		}

		for _, item := range payload.List {
			if item.ReceiptNo == "" || item.ReportName == "" {
				continue
			}
			out = append(out, DayDisclosure{
				StockCode:   item.StockCode,
				ReceiptNo:   item.ReceiptNo,
				ReceiptDate: isoDate,
				Title:       item.ReportName,
				Filer:       item.FilerName,
			})
		}
		if payload.TotalPage <= page {
			break
		}
	}
	return out, pages, nil
}
