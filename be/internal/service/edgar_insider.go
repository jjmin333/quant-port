package service

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
	"github.com/enact/quant-port/be/internal/model"
)

// edgarInsiderMaxFilings bounds how many individual Form 4 XML documents get
// fetched per symbol in one run.
//
// 예전 값은 3이었다. 조회 요청이 직접 EDGAR를 부르던 시절에는 8/min 버킷에서
// 종목당 4호출이 최대치였기 때문인데, 그 상한이 **빈 화면의 원인**이었다:
// 최근 3건이 전부 파생거래(옵션 행사·RSU 수령)면 비파생 표가 비어 있어
// 아무것도 못 보여준다. 대형주는 실제로 자주 그렇다.
//
// 이제 적재는 배치이고 예산은 전용 버킷에서 나가므로, 상한은 "한 종목에 얼마나
// 오래 머물 것인가"의 문제일 뿐이다. 20건이면 첫 적재에서 최근 몇 달을 덮고,
// 이후 실행은 이미 가진 accession을 건너뛰므로 평시 0~2건만 내려받는다.
const edgarInsiderMaxFilings = 20

// InsiderFiling is one Form 4 document's worth of non-derivative transactions,
// carried with the accession number so the ingest can store it under a stable
// key and skip it next time.
type InsiderFiling struct {
	// Key is the SEC accession number (e.g. "0000320193-26-000012").
	Key          string
	Transactions []model.InsiderTransaction
}

// InsiderFetchResult reports what one symbol's fetch actually did, so the
// caller can tell "no filings exist" apart from "we already had them all"
// apart from "every fetch failed".
type InsiderFetchResult struct {
	Filings []InsiderFiling
	// FormsSeen is how many Form 4 filings the submissions feed listed
	// (before the known-key filter), capped at edgarInsiderMaxFilings.
	FormsSeen int
	// Skipped is how many of those were already stored.
	Skipped int
}

// FetchInsiderFilings lists recent Form 4 (change-in-ownership) filings for
// ticker via SEC's per-issuer submissions feed, then downloads the raw
// ownership XML of the ones `known` does not already contain — submissions.json
// carries filing metadata only, not transaction detail.
//
// 증분이 이 함수의 핵심이다. 종목당 고정 비용은 submissions.json 1회뿐이고,
// 나머지는 **새로 올라온 공시 수만큼만** 든다. 첫 적재만 비싸다.
//
// Best-effort per filing: 한 건이 깨져도 나머지는 담는다. 다만 **전부** 실패하면
// 에러다 — 성공으로 처리하면 sync_state에 'empty'가 찍히고, 일시적 장애가
// "이 종목은 내부자 거래가 없다"로 굳는다.
func (e *EdgarClient) FetchInsiderFilings(
	ctx context.Context,
	ticker string,
	known map[string]struct{},
) (InsiderFetchResult, error) {
	var out InsiderFetchResult

	cik, err := e.LookupCIK(ctx, ticker)
	if err != nil {
		return out, err
	}
	e.pace(ctx)
	subs, err := e.fetchSubmissions(ctx, cik)
	if err != nil {
		return out, err
	}

	cikInt := strings.TrimLeft(cik, "0")
	if cikInt == "" {
		cikInt = "0"
	}

	recent := subs.Filings.Recent
	type filingRef struct{ accession, doc string }
	var pending []filingRef
	for i, form := range recent.Form {
		if form != "4" && form != "4/A" {
			continue
		}
		if i >= len(recent.AccessionNumber) || i >= len(recent.PrimaryDocument) {
			continue
		}
		out.FormsSeen++
		accession := recent.AccessionNumber[i]
		if _, ok := known[accession]; ok {
			out.Skipped++
		} else {
			pending = append(pending, filingRef{accession: accession, doc: recent.PrimaryDocument[i]})
		}
		if out.FormsSeen >= edgarInsiderMaxFilings {
			break
		}
	}

	var lastErr error
	failCount := 0
	for _, f := range pending {
		e.pace(ctx)
		txns, err := e.fetchForm4Transactions(ctx, cikInt, f.accession, f.doc)
		if err != nil {
			lastErr = err
			failCount++
			continue
		}
		out.Filings = append(out.Filings, InsiderFiling{Key: f.accession, Transactions: txns})
	}
	if len(pending) > 0 && failCount == len(pending) {
		return out, fmt.Errorf("edgar form4: all %d filing fetches failed: %w", failCount, lastErr)
	}
	return out, nil
}

type edgarSubmissions struct {
	// Sic · SicDescription은 섹터 배치가 읽는다(edgar_sector.go). 같은 응답이라
	// 내부자 적재가 받을 때도 그대로 딸려 온다.
	Sic            string `json:"sic"`
	SicDescription string `json:"sicDescription"`
	Filings        struct {
		Recent struct {
			AccessionNumber []string `json:"accessionNumber"`
			FilingDate      []string `json:"filingDate"`
			Form            []string `json:"form"`
			PrimaryDocument []string `json:"primaryDocument"`
		} `json:"recent"`
	} `json:"filings"`
}

type ownershipDocumentXML struct {
	PeriodOfReport     string                `xml:"periodOfReport"`
	ReportingOwners    []reportingOwnerXML   `xml:"reportingOwner"`
	NonDerivativeTable nonDerivativeTableXML `xml:"nonDerivativeTable"`
}

type reportingOwnerXML struct {
	ReportingOwnerId struct {
		RptOwnerName string `xml:"rptOwnerName"`
	} `xml:"reportingOwnerId"`
	ReportingOwnerRelationship reportingOwnerRelationshipXML `xml:"reportingOwnerRelationship"`
}

type reportingOwnerRelationshipXML struct {
	IsDirector        string `xml:"isDirector"`
	IsOfficer         string `xml:"isOfficer"`
	IsTenPercentOwner string `xml:"isTenPercentOwner"`
	IsOther           string `xml:"isOther"`
	OfficerTitle      string `xml:"officerTitle"`
	OtherText         string `xml:"otherText"`
}

type nonDerivativeTableXML struct {
	Transactions []nonDerivativeTransactionXML `xml:"nonDerivativeTransaction"`
}

type nonDerivativeTransactionXML struct {
	TransactionDate struct {
		Value string `xml:"value"`
	} `xml:"transactionDate"`
	TransactionCoding struct {
		TransactionCode string `xml:"transactionCode"`
	} `xml:"transactionCoding"`
	TransactionAmounts struct {
		TransactionShares struct {
			Value string `xml:"value"`
		} `xml:"transactionShares"`
		TransactionPricePerShare struct {
			Value string `xml:"value"`
		} `xml:"transactionPricePerShare"`
		TransactionAcquiredDisposedCode struct {
			Value string `xml:"value"`
		} `xml:"transactionAcquiredDisposedCode"`
	} `xml:"transactionAmounts"`
	PostTransactionAmounts struct {
		SharesOwnedFollowingTransaction struct {
			Value string `xml:"value"`
		} `xml:"sharesOwnedFollowingTransaction"`
	} `xml:"postTransactionAmounts"`
}

func (e *EdgarClient) fetchSubmissions(ctx context.Context, cik string) (*edgarSubmissions, error) {
	if e.budget != nil {
		if err := e.budget.Check(ctx, e.provider); err != nil {
			return nil, err
		}
	}
	u := fmt.Sprintf("https://data.sec.gov/submissions/CIK%s.json", cik)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", e.cfg.UserAgent)
	req.Header.Set("Accept", "application/json")

	res, err := e.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusTooManyRequests {
		if e.budget != nil {
			e.budget.BlockFor(ctx, e.provider, apibudget.ParseRetryAfter(res.Header.Get("Retry-After")))
		}
		body, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return nil, fmt.Errorf("edgar submissions: %s", strings.TrimSpace(string(body)))
	}
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return nil, fmt.Errorf("edgar submissions: %s", strings.TrimSpace(string(body)))
	}

	var payload edgarSubmissions
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}
	if e.budget != nil {
		e.budget.Record(ctx, e.provider)
	}
	return &payload, nil
}

// fetchForm4Transactions fetches the raw ownership XML for one filing.
// submissions.json's primaryDocument points at the XSLT-styled viewer path
// (e.g. "xslF345X06/form4.xml") — the machine-readable XML sits at the same
// filename directly under the accession folder, so only the basename is used.
func (e *EdgarClient) fetchForm4Transactions(ctx context.Context, cikInt, accession, primaryDoc string) ([]model.InsiderTransaction, error) {
	if e.budget != nil {
		if err := e.budget.Check(ctx, e.provider); err != nil {
			return nil, err
		}
	}
	rawFile := primaryDoc
	if idx := strings.LastIndex(primaryDoc, "/"); idx >= 0 {
		rawFile = primaryDoc[idx+1:]
	}
	accessionNoDashes := strings.ReplaceAll(accession, "-", "")
	u := fmt.Sprintf("https://www.sec.gov/Archives/edgar/data/%s/%s/%s", cikInt, accessionNoDashes, rawFile)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", e.cfg.UserAgent)
	req.Header.Set("Accept", "application/xml,text/xml")

	res, err := e.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusTooManyRequests {
		if e.budget != nil {
			e.budget.BlockFor(ctx, e.provider, apibudget.ParseRetryAfter(res.Header.Get("Retry-After")))
		}
		return nil, fmt.Errorf("edgar form4: rate limited")
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("edgar form4: status %d", res.StatusCode)
	}

	var doc ownershipDocumentXML
	if err := xml.NewDecoder(res.Body).Decode(&doc); err != nil {
		return nil, err
	}
	if e.budget != nil {
		e.budget.Record(ctx, e.provider)
	}

	var reporterName, relation string
	if len(doc.ReportingOwners) > 0 {
		owner := doc.ReportingOwners[0]
		reporterName = strings.TrimSpace(owner.ReportingOwnerId.RptOwnerName)
		relation = edgarInsiderRelation(owner.ReportingOwnerRelationship)
	}

	out := make([]model.InsiderTransaction, 0, len(doc.NonDerivativeTable.Transactions))
	for _, t := range doc.NonDerivativeTable.Transactions {
		date := strings.TrimSpace(t.TransactionDate.Value)
		if date == "" {
			date = doc.PeriodOfReport
		}
		shares := parseEdgarFloatPtr(t.TransactionAmounts.TransactionShares.Value)
		var sharesChange *int64
		if shares != nil {
			v := int64(*shares)
			if t.TransactionAmounts.TransactionAcquiredDisposedCode.Value == "D" {
				v = -v
			}
			sharesChange = &v
		}
		out = append(out, model.InsiderTransaction{
			Date:             date,
			ReporterName:     reporterName,
			Relation:         relation,
			TransactionType:  edgarTransactionCodeLabel(t.TransactionCoding.TransactionCode),
			SharesChange:     sharesChange,
			SharesOwnedAfter: parseEdgarIntPtr(t.PostTransactionAmounts.SharesOwnedFollowingTransaction.Value),
			PricePerShare:    parseEdgarFloatPtr(t.TransactionAmounts.TransactionPricePerShare.Value),
			Source:           "edgar",
		})
	}
	return out, nil
}

func edgarInsiderRelation(rel reportingOwnerRelationshipXML) string {
	isTrue := func(v string) bool {
		v = strings.TrimSpace(strings.ToLower(v))
		return v == "true" || v == "1"
	}
	var parts []string
	if isTrue(rel.IsOfficer) {
		if title := strings.TrimSpace(rel.OfficerTitle); title != "" {
			parts = append(parts, title)
		} else {
			parts = append(parts, "Officer")
		}
	}
	if isTrue(rel.IsDirector) {
		parts = append(parts, "Director")
	}
	if isTrue(rel.IsTenPercentOwner) {
		parts = append(parts, "10% Owner")
	}
	if isTrue(rel.IsOther) {
		if other := strings.TrimSpace(rel.OtherText); other != "" {
			parts = append(parts, other)
		} else {
			parts = append(parts, "Other")
		}
	}
	return strings.Join(parts, " · ")
}

// edgarTransactionCodeLabels maps SEC Form 4 Table I/II transaction codes to
// short Korean labels. Codes are the fixed set defined by SEC Form 4
// instructions — safe to hardcode.
var edgarTransactionCodeLabels = map[string]string{
	"P": "매수", "S": "매도", "A": "취득(부여)", "D": "처분",
	"F": "세금원천징수", "I": "재량거래", "M": "파생상품 행사", "C": "전환",
	"E": "만기(공매도)", "H": "만기(매수)", "O": "옵션행사(외가격)", "X": "옵션행사(등가/내가격)",
	"G": "증여", "L": "소액취득", "W": "상속/유증", "Z": "의결권신탁 입출고",
	"J": "기타 취득/처분", "K": "스왑거래", "U": "공개매수 처분", "V": "자발적 보고",
}

func edgarTransactionCodeLabel(code string) string {
	code = strings.TrimSpace(code)
	if label, ok := edgarTransactionCodeLabels[code]; ok {
		return label
	}
	return code
}

func parseEdgarFloatPtr(raw string) *float64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return nil
	}
	return &v
}

func parseEdgarIntPtr(raw string) *int64 {
	f := parseEdgarFloatPtr(raw)
	if f == nil {
		return nil
	}
	v := int64(*f)
	return &v
}
