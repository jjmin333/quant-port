package handler

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"

	"github.com/enact/quant-port/be/internal/authctx"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/pkg/response"
)

/*
WealthHandler — 순자산(자산·부채)과 공공데이터 조회 (2026-10-01). 외부 호출은 업비트 원화 시세 1분 캐시뿐이다.

	보호   GET /wealth · POST/PUT/DELETE /wealth/assets[/{id}] · GET /wealth/assets/{id}
	       POST/PUT/DELETE /wealth/liabilities[/{id}]
	공개   GET /realestate/regions · /realestate/complexes · /realestate/complex · /realestate/market
	       GET /macro/rates · /auctions/onbid · /auctions/onbid/stats · /crypto/markets
*/
type WealthHandler struct {
	wealth *repository.WealthRepo
	data   *repository.PublicDataRepo
	values *service.WealthValuer
	intel  *repository.AssetIntelRepo
	watch  *repository.AssetWatchRepo
}

func NewWealthHandler(wealth *repository.WealthRepo, data *repository.PublicDataRepo, values *service.WealthValuer, intel *repository.AssetIntelRepo) *WealthHandler {
	return &WealthHandler{wealth: wealth, data: data, values: values, intel: intel}
}

var (
	assetTypes     = map[string]bool{"real_estate": true, "deposit": true, "saving": true, "bond": true, "crypto": true, "gold": true, "vehicle": true, "art": true, "other": true}
	propertyTypes  = map[string]bool{"apt": true, "offi": true, "rh": true, "sh": true, "land": true, "nrg": true, "indu": true, "silv": true}
	liabilityKinds = map[string]bool{"mortgage": true, "jeonse_deposit": true, "credit": true, "other": true}
	repayTypes     = map[string]bool{"": true, "equal_payment": true, "equal_principal": true, "bullet": true}
	rateTypes      = map[string]bool{"": true, "fixed": true, "variable": true, "mixed": true}
	lawdRe         = regexp.MustCompile(`^\d{5}$`)
	cryptoMarketRe = regexp.MustCompile(`^KRW-[A-Z0-9]{1,15}$`)
	regexpDigits2  = regexp.MustCompile(`^\d{2}$`)
)

const (
	maxWealthRows = 200
	maxMoney      = 1e15
)

func badRequest(w http.ResponseWriter, msg string) { response.Error(w, http.StatusBadRequest, msg) }

func validDate(s *string) bool {
	if s == nil {
		return true
	}
	t, err := time.Parse("2006-01-02", *s)
	return err == nil && t.Year() >= 1900 && t.Year() <= 2200
}

func validMoney(v *float64, allowZero bool) bool {
	if v == nil {
		return true
	}
	if math.IsNaN(*v) || math.IsInf(*v, 0) || *v < 0 || *v > maxMoney {
		return false
	}
	return allowZero || *v > 0
}

func validRate(v float64) bool { return v >= 0 && v <= 30 && !math.IsNaN(v) }

// ── 목록 ────────────────────────────────────────────────────────────────────

// AssetView — 자산 + 지금 평가 + 손익. 금액은 내 지분 기준(전체 × 지분율).
type AssetView struct {
	repository.UserAsset
	Valuation     *repository.AssetValuation `json:"valuation,omitempty"`
	ValuationNote string                     `json:"valuationNote,omitempty"`
	MyValue       *float64                   `json:"myValue,omitempty"`
	MyCost        *float64                   `json:"myCost,omitempty"`
	Pnl           *float64                   `json:"pnl,omitempty"`
	ReturnPct     *float64                   `json:"returnPct,omitempty"`
	AnnualizedPct *float64                   `json:"annualizedPct,omitempty"`
	Live          any                        `json:"live,omitempty"`
}

// LiabilityView — 부채 + 약정 기준 잔액·월 상환액.
type LiabilityView struct {
	repository.UserLiability
	ScheduledBalance *float64 `json:"scheduledBalance,omitempty"`
	MonthlyPayment   *float64 `json:"monthlyPayment,omitempty"`
}

func (h *WealthHandler) assetView(r *http.Request, a repository.UserAsset, stored map[int64]repository.AssetValuation) AssetView {
	v := AssetView{UserAsset: a}
	// 부동산·직접 입력은 저장된 평가(배치·저장 시 계산), 나머지는 지금 계산(가볍다).
	var val *repository.AssetValuation
	if s, ok := stored[a.ID]; ok && (a.AssetType == "real_estate") {
		val = &s
	} else {
		got, live, err := h.values.Valuate(r.Context(), a, time.Now())
		if err == nil {
			got.Inputs = nil
			val, v.Live = &got, live
		} else if s, ok := stored[a.ID]; ok {
			val = &s
		} else {
			v.ValuationNote = err.Error()
		}
	}
	v.Valuation = val
	if val == nil {
		return v
	}
	ratio := a.OwnershipRatio
	if ratio <= 0 {
		ratio = 1
	}
	my := math.Round(val.Value * ratio)
	v.MyValue = &my
	cost := costOf(a)
	if cost != nil {
		c := math.Round(*cost * ratio)
		v.MyCost = &c
		if c > 0 {
			pnl := my - c
			ret := pnl / c * 100
			v.Pnl, v.ReturnPct = &pnl, &ret
			if a.AcquiredAt != nil {
				if t, err := time.Parse("2006-01-02", *a.AcquiredAt); err == nil {
					years := time.Since(t).Hours() / 24 / 365
					if years >= 1 && my > 0 {
						ann := (math.Pow(my/c, 1/years) - 1) * 100
						v.AnnualizedPct = &ann
					}
				}
			}
		}
	}
	return v
}

// costOf — 원가: 취득가(+부동산 취득 부대비용), 예금은 원금, 적금은 낸 돈은 평가에 포함되므로 원금 합.
func costOf(a repository.UserAsset) *float64 {
	switch a.AssetType {
	case "deposit", "bond":
		var d service.FixedDetail
		_ = json.Unmarshal(a.Detail, &d)
		if a.AcquiredPrice != nil {
			return a.AcquiredPrice
		}
		if d.Principal > 0 {
			return &d.Principal
		}
	case "saving":
		var d service.FixedDetail
		_ = json.Unmarshal(a.Detail, &d)
		start, err := time.Parse("2006-01-02", d.StartDate)
		if err != nil || d.Monthly <= 0 {
			return nil
		}
		n := 0
		now := time.Now()
		for i := 0; i < d.Months; i++ {
			if start.AddDate(0, i, 0).After(now) {
				break
			}
			n++
		}
		c := d.Monthly * float64(n)
		return &c
	case "real_estate":
		if a.AcquiredPrice == nil {
			return nil
		}
		var d service.ReDetail
		_ = json.Unmarshal(a.Detail, &d)
		c := *a.AcquiredPrice
		if d.AcquisitionCost != nil {
			c += *d.AcquisitionCost
		}
		return &c
	}
	return a.AcquiredPrice
}

// Get serves GET /wealth — 자산·부채 전부와 평가.
func (h *WealthHandler) Get(w http.ResponseWriter, r *http.Request) {
	uid, err := authctx.RequireUserID(r.Context())
	if err != nil {
		response.Error(w, http.StatusUnauthorized, "login required")
		return
	}
	assets, err := h.wealth.ListAssets(r.Context(), uid)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	stored, err := h.wealth.LatestValuations(r.Context(), uid)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	views := make([]AssetView, 0, len(assets))
	for _, a := range assets {
		views = append(views, h.assetView(r, a, stored))
	}
	liabs, err := h.wealth.ListLiabilities(r.Context(), uid)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	lv := make([]LiabilityView, 0, len(liabs))
	for _, l := range liabs {
		lv = append(lv, liabilityView(l))
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]any{"assets": views, "liabilities": lv}})
}

func liabilityView(l repository.UserLiability) LiabilityView {
	v := LiabilityView{UserLiability: l}
	if l.Rate != nil && l.StartedAt != nil && l.MaturityAt != nil {
		start, _ := time.Parse("2006-01-02", *l.StartedAt)
		mat, _ := time.Parse("2006-01-02", *l.MaturityAt)
		if bal, pay, ok := service.ScheduledLoan(l.Principal, *l.Rate, l.RepaymentType, start, mat, time.Now()); ok {
			v.ScheduledBalance, v.MonthlyPayment = &bal, &pay
		}
	}
	return v
}

// ── 자산 쓰기 ────────────────────────────────────────────────────────────────

type assetBody struct {
	AssetType       string          `json:"assetType"`
	Name            string          `json:"name"`
	Quantity        *float64        `json:"quantity"`
	AcquiredAt      *string         `json:"acquiredAt"`
	AcquiredPrice   *float64        `json:"acquiredPrice"`
	OwnershipRatio  *float64        `json:"ownershipRatio"`
	ValuationMethod string          `json:"valuationMethod"`
	UserValue       *float64        `json:"userValue"`
	Detail          json.RawMessage `json:"detail"`
	Memo            string          `json:"memo"`
}

// validateAsset checks a body and builds the row. 실패면 사용자에게 보일 이유.
func validateAsset(b assetBody) (repository.UserAsset, string) {
	a := repository.UserAsset{AssetType: b.AssetType, Currency: "KRW", OwnershipRatio: 1, Memo: strings.TrimSpace(b.Memo)}
	if !assetTypes[b.AssetType] {
		return a, "알 수 없는 자산 종류입니다"
	}
	a.Name = strings.TrimSpace(b.Name)
	if a.Name == "" || utf8.RuneCountInString(a.Name) > 100 {
		return a, "이름은 1~100자로 입력하세요"
	}
	if utf8.RuneCountInString(a.Memo) > 300 {
		return a, "메모는 300자까지입니다"
	}
	if b.OwnershipRatio != nil {
		if *b.OwnershipRatio <= 0 || *b.OwnershipRatio > 1 {
			return a, "지분율은 0보다 크고 100% 이하여야 합니다"
		}
		a.OwnershipRatio = *b.OwnershipRatio
	}
	if !validDate(b.AcquiredAt) {
		return a, "취득일 형식이 올바르지 않습니다(YYYY-MM-DD)"
	}
	if b.AcquiredAt != nil && *b.AcquiredAt > time.Now().Format("2006-01-02") {
		return a, "취득일이 미래입니다"
	}
	if !validMoney(b.AcquiredPrice, true) || !validMoney(b.UserValue, false) {
		return a, "금액이 올바르지 않습니다"
	}
	a.AcquiredAt, a.AcquiredPrice, a.UserValue = b.AcquiredAt, b.AcquiredPrice, b.UserValue
	if len(b.Detail) == 0 || string(b.Detail) == "null" {
		b.Detail = json.RawMessage(`{}`)
	}
	if len(b.Detail) > 4000 {
		return a, "상세 정보가 너무 깁니다"
	}
	switch b.AssetType {
	case "real_estate":
		var d service.ReDetail
		if err := json.Unmarshal(b.Detail, &d); err != nil {
			return a, "부동산 정보 형식이 올바르지 않습니다"
		}
		a.ValuationMethod = "comparable"
		if b.ValuationMethod == "user_estimated" {
			a.ValuationMethod = "user_estimated"
			if b.UserValue == nil {
				return a, "직접 입력 평가는 현재가가 필요합니다"
			}
		} else {
			if !propertyTypes[d.PropertyType] {
				return a, "부동산 종류를 고르세요"
			}
			if !lawdRe.MatchString(d.LawdCd) {
				return a, "지역(시군구)을 고르세요"
			}
			if d.Area <= 0 || d.Area > 1e6 {
				return a, "면적(㎡)을 입력하세요"
			}
		}
		if !validMoney(d.LeaseDeposit, true) || !validMoney(d.MonthlyRent, true) || !validMoney(d.AcquisitionCost, true) {
			return a, "금액이 올바르지 않습니다"
		}
		if utf8.RuneCountInString(d.ComplexKey) > 120 || utf8.RuneCountInString(d.Address) > 200 {
			return a, "주소가 너무 깁니다"
		}
		b.Detail, _ = json.Marshal(d)
	case "deposit", "saving", "bond":
		var d service.FixedDetail
		if err := json.Unmarshal(b.Detail, &d); err != nil {
			return a, "상품 정보 형식이 올바르지 않습니다"
		}
		if !validRate(d.Rate) {
			return a, "금리는 0~30%로 입력하세요"
		}
		start, maturity := &d.StartDate, &d.Maturity
		if d.StartDate == "" {
			start = nil
		}
		if d.Maturity == "" {
			maturity = nil
		}
		if !validDate(start) || !validDate(maturity) {
			return a, "날짜 형식이 올바르지 않습니다(YYYY-MM-DD)"
		}
		if start != nil && maturity != nil && d.Maturity < d.StartDate {
			return a, "만기가 시작일보다 앞입니다"
		}
		if d.TaxRate != nil && (*d.TaxRate < 0 || *d.TaxRate > 50) {
			return a, "세율은 0~50%입니다"
		}
		switch b.AssetType {
		case "deposit":
			a.ValuationMethod = "fixed_income"
			if !validMoney(&d.Principal, false) || start == nil {
				return a, "원금과 가입일을 입력하세요"
			}
		case "saving":
			a.ValuationMethod = "fixed_income"
			if !validMoney(&d.Monthly, false) || d.Months < 1 || d.Months > 600 || start == nil {
				return a, "월 납입액·기간·가입일을 입력하세요"
			}
		case "bond":
			a.ValuationMethod = "bond"
			if !validMoney(&d.Principal, false) || maturity == nil {
				return a, "액면 총액과 만기일을 입력하세요"
			}
			if d.CouponFreq != 0 && d.CouponFreq != 1 && d.CouponFreq != 2 && d.CouponFreq != 4 && d.CouponFreq != 12 {
				return a, "이자 지급 횟수는 0·1·2·4·12 중 하나입니다"
			}
			if d.Credit != "" && d.Credit != "gov" && d.Credit != "AA" && d.Credit != "BBB" {
				return a, "신용등급은 국채·AA·BBB 중 하나입니다"
			}
		}
		if utf8.RuneCountInString(d.Institution) > 60 {
			return a, "기관명이 너무 깁니다"
		}
		b.Detail, _ = json.Marshal(d)
	case "crypto", "gold":
		var d service.MarketDetail
		if err := json.Unmarshal(b.Detail, &d); err != nil {
			return a, "형식이 올바르지 않습니다"
		}
		if b.Quantity == nil || *b.Quantity <= 0 || *b.Quantity > 1e12 || math.IsNaN(*b.Quantity) {
			return a, "수량을 입력하세요"
		}
		a.Quantity = b.Quantity
		a.ValuationMethod = "market_price"
		if b.AssetType == "crypto" {
			d.Market = strings.ToUpper(strings.TrimSpace(d.Market))
			if !cryptoMarketRe.MatchString(d.Market) {
				return a, "코인을 고르세요(업비트 원화 마켓)"
			}
		} else {
			d.Unit = "g"
		}
		b.Detail, _ = json.Marshal(d)
	default: // vehicle · art · other
		var d service.OtherDetail
		if err := json.Unmarshal(b.Detail, &d); err != nil {
			return a, "형식이 올바르지 않습니다"
		}
		if d.DepreciationPct < 0 || d.DepreciationPct > 50 {
			return a, "감가율은 0~50%입니다"
		}
		if b.UserValue == nil {
			return a, "현재가를 입력하세요"
		}
		a.ValuationMethod = "user_estimated"
		b.Detail, _ = json.Marshal(d)
	}
	a.Detail = b.Detail
	return a, ""
}

func (h *WealthHandler) SaveAsset(w http.ResponseWriter, r *http.Request) {
	uid, err := authctx.RequireUserID(r.Context())
	if err != nil {
		response.Error(w, http.StatusUnauthorized, "login required")
		return
	}
	var b assetBody
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&b); err != nil {
		badRequest(w, "잘못된 요청입니다")
		return
	}
	a, msg := validateAsset(b)
	if msg != "" {
		badRequest(w, msg)
		return
	}
	a.UserID = uid
	if idStr := chi.URLParam(r, "id"); idStr != "" {
		a.ID, err = strconv.ParseInt(idStr, 10, 64)
		if err != nil || a.ID <= 0 {
			badRequest(w, "invalid id")
			return
		}
	} else {
		list, err := h.wealth.ListAssets(r.Context(), uid)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		if len(list) >= maxWealthRows {
			badRequest(w, "자산은 200개까지 등록할 수 있습니다")
			return
		}
	}
	found, err := h.wealth.SaveAsset(r.Context(), &a)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !found {
		response.Error(w, http.StatusNotFound, "asset not found")
		return
	}
	// 그 지역은 이제 12종 전부 적재한다(다음 배치부터).
	if a.AssetType == "real_estate" {
		var d service.ReDetail
		_ = json.Unmarshal(a.Detail, &d)
		if d.LawdCd != "" {
			_ = h.data.PromoteRegion(r.Context(), d.LawdCd)
		}
	}
	saved, err := h.wealth.GetAsset(r.Context(), uid, a.ID)
	if err != nil || saved == nil {
		response.Error(w, http.StatusInternalServerError, "reload failed")
		return
	}
	// 저장한 순간의 평가를 오늘 줄로 남긴다 — 화면이 바로 숫자를 보인다.
	if val, _, err := h.values.Valuate(r.Context(), *saved, time.Now()); err == nil {
		_ = h.wealth.UpsertValuation(r.Context(), val)
	}
	stored, _ := h.wealth.LatestValuations(r.Context(), uid)
	response.JSON(w, http.StatusOK, map[string]any{"data": h.assetView(r, *saved, stored)})
}

func (h *WealthHandler) DeleteAsset(w http.ResponseWriter, r *http.Request) {
	uid, err := authctx.RequireUserID(r.Context())
	if err != nil {
		response.Error(w, http.StatusUnauthorized, "login required")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		badRequest(w, "invalid id")
		return
	}
	if err := h.wealth.DeleteAsset(r.Context(), uid, id); err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]any{"id": id, "deleted": true}})
}

// AssetDetail serves GET /wealth/assets/{id} — 평가 근거 · 추이 · (부동산) 지역 시장.
func (h *WealthHandler) AssetDetail(w http.ResponseWriter, r *http.Request) {
	uid, err := authctx.RequireUserID(r.Context())
	if err != nil {
		response.Error(w, http.StatusUnauthorized, "login required")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		badRequest(w, "invalid id")
		return
	}
	a, err := h.wealth.GetAsset(r.Context(), uid, id)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if a == nil {
		response.Error(w, http.StatusNotFound, "asset not found")
		return
	}
	history, err := h.wealth.ValuationHistory(r.Context(), id, 400)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := map[string]any{"history": history}
	val, explain, verr := h.values.Valuate(r.Context(), *a, time.Now())
	if verr == nil {
		// 상세가 계산한 오늘 값을 저장한다 — 목록(저장된 평가)과 상세가 같은 숫자를 보이게.
		_ = h.wealth.UpsertValuation(r.Context(), val)
		val.Inputs = nil
		out["now"], out["explain"] = val, explain
	} else {
		out["error"] = verr.Error()
	}
	stored, _ := h.wealth.LatestValuations(r.Context(), uid)
	out["asset"] = h.assetView(r, *a, stored)
	if a.AssetType == "real_estate" {
		var d service.ReDetail
		_ = json.Unmarshal(a.Detail, &d)
		if d.LawdCd != "" {
			out["market"] = h.regionMarket(r, d.LawdCd, d.PropertyType)
		}
		if d.ComplexKey != "" {
			since := time.Now().AddDate(-3, 0, 0)
			sales, _ := h.data.ComplexTrades(r.Context(), d.ComplexKey, "sale", since)
			rents, _ := h.data.ComplexTrades(r.Context(), d.ComplexKey, "rent", time.Now().AddDate(-1, 0, 0))
			out["sales"], out["rents"] = sales, rents
		}
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": out})
}

// ── 부채 ────────────────────────────────────────────────────────────────────

type liabilityBody struct {
	AssetID       *int64   `json:"assetId"`
	Kind          string   `json:"kind"`
	Name          string   `json:"name"`
	Principal     float64  `json:"principal"`
	Balance       float64  `json:"balance"`
	Rate          *float64 `json:"rate"`
	RateType      string   `json:"rateType"`
	RepaymentType string   `json:"repaymentType"`
	StartedAt     *string  `json:"startedAt"`
	MaturityAt    *string  `json:"maturityAt"`
	Memo          string   `json:"memo"`
}

func (h *WealthHandler) SaveLiability(w http.ResponseWriter, r *http.Request) {
	uid, err := authctx.RequireUserID(r.Context())
	if err != nil {
		response.Error(w, http.StatusUnauthorized, "login required")
		return
	}
	var b liabilityBody
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&b); err != nil {
		badRequest(w, "잘못된 요청입니다")
		return
	}
	l := repository.UserLiability{UserID: uid, AssetID: b.AssetID, Kind: b.Kind, Name: strings.TrimSpace(b.Name),
		Principal: b.Principal, Balance: b.Balance, Rate: b.Rate, RateType: b.RateType, RepaymentType: b.RepaymentType,
		StartedAt: b.StartedAt, MaturityAt: b.MaturityAt, Memo: strings.TrimSpace(b.Memo)}
	switch {
	case !liabilityKinds[l.Kind]:
		badRequest(w, "부채 종류를 고르세요")
		return
	case l.Name == "" || utf8.RuneCountInString(l.Name) > 100:
		badRequest(w, "이름은 1~100자로 입력하세요")
		return
	case !validMoney(&l.Principal, true) || !validMoney(&l.Balance, true):
		badRequest(w, "금액이 올바르지 않습니다")
		return
	case l.Rate != nil && !validRate(*l.Rate):
		badRequest(w, "금리는 0~30%로 입력하세요")
		return
	case !repayTypes[l.RepaymentType] || !rateTypes[l.RateType]:
		badRequest(w, "상환 방식 또는 금리 유형이 올바르지 않습니다")
		return
	case !validDate(l.StartedAt) || !validDate(l.MaturityAt):
		badRequest(w, "날짜 형식이 올바르지 않습니다(YYYY-MM-DD)")
		return
	case l.StartedAt != nil && l.MaturityAt != nil && *l.MaturityAt < *l.StartedAt:
		badRequest(w, "만기가 시작일보다 앞입니다")
		return
	case utf8.RuneCountInString(l.Memo) > 300:
		badRequest(w, "메모는 300자까지입니다")
		return
	}
	if idStr := chi.URLParam(r, "id"); idStr != "" {
		l.ID, err = strconv.ParseInt(idStr, 10, 64)
		if err != nil || l.ID <= 0 {
			badRequest(w, "invalid id")
			return
		}
	} else {
		list, err := h.wealth.ListLiabilities(r.Context(), uid)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		if len(list) >= maxWealthRows {
			badRequest(w, "부채는 200개까지 등록할 수 있습니다")
			return
		}
	}
	found, err := h.wealth.SaveLiability(r.Context(), &l)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !found {
		response.Error(w, http.StatusNotFound, "liability not found")
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": liabilityView(l)})
}

func (h *WealthHandler) DeleteLiability(w http.ResponseWriter, r *http.Request) {
	uid, err := authctx.RequireUserID(r.Context())
	if err != nil {
		response.Error(w, http.StatusUnauthorized, "login required")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		badRequest(w, "invalid id")
		return
	}
	if err := h.wealth.DeleteLiability(r.Context(), uid, id); err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]any{"id": id, "deleted": true}})
}

// ── 부동산 조회 (공개) ───────────────────────────────────────────────────────

func (h *WealthHandler) Regions(w http.ResponseWriter, r *http.Request) {
	list, err := h.data.Regions(r.Context())
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": list})
}

func (h *WealthHandler) Complexes(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	lawd, kind := q.Get("lawdCd"), q.Get("kind")
	if kind == "" {
		kind = "apt"
	}
	if !propertyTypes[kind] || (lawd != "" && !lawdRe.MatchString(lawd)) {
		badRequest(w, "invalid kind or lawdCd")
		return
	}
	text := strings.TrimSpace(q.Get("q"))
	// 지역 없이 찾는 것은 아파트 이름 검색뿐이다 — 단지 목록(re_complexes, 4만여 행)을 보므로 전국 실거래를 훑지 않는다.
	// 그 밖(빌라·오피스텔 등)은 실거래 표를 훑으므로 시군구를 받는다.
	if lawd == "" && (kind != "apt" || utf8.RuneCountInString(text) < 2) {
		badRequest(w, "지역(시군구)을 고르세요")
		return
	}
	if utf8.RuneCountInString(text) > 40 {
		badRequest(w, "검색어가 너무 깁니다")
		return
	}
	list, err := h.data.SearchComplexes(r.Context(), lawd, kind, text, 40)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": list})
}

// Complex serves GET /realestate/complex?key=&area= — 한 단지의 매매·전월세와 (면적을 주면) 추정가.
func (h *WealthHandler) Complex(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	key := q.Get("key")
	if key == "" || utf8.RuneCountInString(key) > 120 {
		badRequest(w, "key required")
		return
	}
	since := time.Now().AddDate(-3, 0, 0)
	sales, err := h.data.ComplexTrades(r.Context(), key, "sale", since)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	rents, err := h.data.ComplexTrades(r.Context(), key, "rent", time.Now().AddDate(-1, 0, 0))
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	// 면적 목록(거래 많은 순).
	counts := map[float64]int{}
	for _, t := range sales {
		counts[math.Round(t.Area*100)/100]++
	}
	type areaCount struct {
		Area  float64 `json:"area"`
		Count int     `json:"count"`
	}
	areas := []areaCount{}
	for a, c := range counts {
		areas = append(areas, areaCount{a, c})
	}
	sort.Slice(areas, func(i, j int) bool { return areas[i].Count > areas[j].Count })
	out := map[string]any{"sales": sales, "rents": rents, "areas": areas}
	out["score"], out["note"] = h.complexIntel(r, key)
	if area, err := strconv.ParseFloat(q.Get("area"), 64); err == nil && area > 0 {
		today := time.Now()
		idx := (service.IndexAt)(nil)
		if len(sales) > 0 {
			idx = h.values.RegionIndexFor(r.Context(), q.Get("lawdCd"))
		}
		if est, ok := service.EstimateComparable(sales, area, today, idx); ok {
			if m, n := service.JeonseEstimate(rents, area, today); n > 0 {
				out["jeonse"] = map[string]any{"median": m, "n": n}
			}
			out["estimate"] = est
		}
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": out})
}

// Market serves GET /realestate/market?lawdCd=|sido= — 지역 지수·거래량. 둘 다 비면 전국.
func (h *WealthHandler) Market(w http.ResponseWriter, r *http.Request) {
	lawd, sido := r.URL.Query().Get("lawdCd"), r.URL.Query().Get("sido")
	if (lawd != "" && !lawdRe.MatchString(lawd)) || (sido != "" && !regexpDigits2.MatchString(sido)) {
		badRequest(w, "invalid lawdCd")
		return
	}
	if lawd == "" {
		response.JSON(w, http.StatusOK, map[string]any{"data": h.areaMarket(r, sido)})
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": h.regionMarket(r, lawd, "apt")})
}

// areaMarket — 전국("") 또는 시도의 R-ONE 지수 + 월별 거래(asset_scores가 facts에 미리 합쳐 둔 것).
func (h *WealthHandler) areaMarket(r *http.Request, sido string) map[string]any {
	ctx := r.Context()
	name, short, kind, key := "전국", "전국", "re_nation", "00"
	if sido != "" {
		kind, key = "re_sido", sido
		regions, _ := h.data.Regions(ctx)
		for _, g := range regions {
			if strings.HasPrefix(g.LawdCd, sido) {
				name, short = g.Sido, service.SidoShort(g.Sido)
				break
			}
		}
	}
	out := map[string]any{"lawdCd": "", "sido": sido, "name": name, "indices": h.roneSeries(ctx, func(table string) (int, bool) {
		cls, ok, _ := h.data.IndexClsByName(ctx, table, short, short)
		return cls, ok
	}), "source": "한국부동산원 부동산통계정보(R-ONE) · 국토교통부 실거래가"}
	if s, err := h.intel.Score(ctx, kind, key); err == nil && s != nil {
		var f struct {
			Months []repository.RegionMonth `json:"months"`
		}
		if json.Unmarshal(s.Facts, &f) == nil {
			out["months"] = f.Months
		}
	}
	return out
}

// regionMarket — 지역의 R-ONE 지수 묶음 + 우리 실거래 월별 통계.
func (h *WealthHandler) regionMarket(r *http.Request, lawdCd, kind string) map[string]any {
	ctx := r.Context()
	if lawdCd == "" {
		return h.areaMarket(r, "")
	}
	sido, name := "", "전국"
	regions, _ := h.data.Regions(ctx)
	for _, g := range regions {
		if g.LawdCd == lawdCd {
			sido, name = g.Sido, g.Name
		}
	}
	series := h.roneSeries(ctx, func(table string) (int, bool) { return service.RoneClsFor(ctx, h.data, table, sido, name) })
	out := map[string]any{"lawdCd": lawdCd, "sido": sido, "name": name, "indices": series,
		"source": "한국부동산원 부동산통계정보(R-ONE) · 국토교통부 실거래가"}
	if kind == "" {
		kind = "apt"
	}
	months, _ := h.data.RegionMonthly(ctx, lawdCd, kind)
	out["months"] = months
	return out
}

// roneSeries — R-ONE 표마다 clsOf가 고른 분류의 5년 시계열.
func (h *WealthHandler) roneSeries(ctx context.Context, clsOf func(table string) (int, bool)) []map[string]any {
	since := time.Now().AddDate(-5, 0, 0)
	series := []map[string]any{}
	for _, t := range service.RoneTables {
		cls, ok := clsOf(t.ID)
		if !ok {
			continue
		}
		items, _ := h.data.IndexItems(ctx, t.ID, cls)
		itm := 0
		for id, n := range items {
			if itm == 0 || n == "지수" || n == "공실률" || strings.Contains(n, "전환율") {
				itm = id
			}
		}
		pts, err := h.data.IndexSeries(ctx, t.ID, cls, itm, since)
		if err != nil || len(pts) == 0 {
			continue
		}
		series = append(series, map[string]any{"key": t.Key, "label": t.Label, "unit": t.Unit, "cycle": t.Cycle, "points": pts})
	}
	return series
}

// ── 금리 · 공매 · 코인 (공개) ────────────────────────────────────────────────

// Rates serves GET /macro/rates — 계열별 최신·직전 값, 국고채 곡선, 주요 계열 추이.
func (h *WealthHandler) Rates(w http.ResponseWriter, r *http.Request) {
	latest, err := h.data.LatestRates(r.Context())
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	items := []map[string]any{}
	curve := []map[string]any{}
	for _, d := range service.RateSeriesDefs {
		p, ok := latest[d.ID()]
		if !ok {
			continue
		}
		it := map[string]any{"key": d.Key, "label": d.Label, "cycle": d.Cycle, "date": p[0].Date, "value": p[0].Value}
		if p[1].Date != "" {
			it["prev"], it["prevDate"] = p[1].Value, p[1].Date
		}
		items = append(items, it)
		if d.Years > 0 {
			curve = append(curve, map[string]any{"years": d.Years, "value": p[0].Value})
		}
	}
	trend := map[string]any{}
	since := time.Now().AddDate(-5, 0, 0)
	for _, d := range service.RateSeriesDefs {
		switch d.Key {
		case "base", "deposit1y", "mortgage", "creditLoan", "jeonseLoan":
			pts, _ := h.data.RateSeries(r.Context(), d.ID(), since)
			trend[d.Key] = pts
		case "ktb3", "ktb10":
			// 일별은 무겁다 — 월말 값만.
			pts, _ := h.data.RateSeries(r.Context(), d.ID(), since)
			trend[d.Key] = monthEnds(pts)
		}
	}
	data := map[string]any{"items": items, "curve": curve, "trend": trend, "source": "한국은행 경제통계시스템(ECOS)"}
	// 금리 브리핑(홈과 같은 글, 2026-10-03 F4) — 없으면 빠진다.
	if b, ok := h.homeBriefing(r.Context())["rates"]; ok {
		data["brief"] = b
	}
	if notes, err := h.intel.Notes(r.Context(), "rates", []string{"kr"}); err == nil {
		if n, ok := notes["kr"]; ok {
			data["note"] = n
		}
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": data})
}

func monthEnds(pts []repository.IndexPoint) []repository.IndexPoint {
	out := []repository.IndexPoint{}
	for i, p := range pts {
		if i == len(pts)-1 || pts[i+1].Date[:7] != p.Date[:7] {
			out = append(out, p)
		}
	}
	return out
}

// Onbid serves GET /auctions/onbid.
func (h *WealthHandler) Onbid(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 || page > 500 {
		page = 1
	}
	minFails, _ := strconv.Atoi(q.Get("minFails"))
	maxRatio, _ := strconv.ParseFloat(q.Get("maxRatio"), 64)
	// pageSize — 기본 30. 화면이 같은 건물 호실을 묶을 때(2026-10-03 R4)는 120을 받아 묶은 뒤에도 카드가 남게.
	size := 30
	if v := q.Get("pageSize"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 10 || n > 120 {
			badRequest(w, "pageSize는 10~120입니다")
			return
		}
		size = n
	}
	query := repository.OnbidQuery{
		Status: q.Get("status"), Sido: strings.TrimSpace(q.Get("sido")), Usage: strings.TrimSpace(q.Get("usage")),
		MinFails: minFails, MaxRatio: maxRatio, Method: q.Get("method"), Sort: q.Get("sort"), Limit: size, Offset: (page - 1) * size,
	}
	if utf8.RuneCountInString(query.Usage) > 20 || utf8.RuneCountInString(query.Sido) > 10 {
		badRequest(w, "검색어가 너무 깁니다")
		return
	}
	items, total, err := h.data.ListOnbid(r.Context(), query)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"items": h.enrichOnbid(r, items), "total": total, "page": page, "pageSize": size, "source": "한국자산관리공사 온비드",
	}})
}

func (h *WealthHandler) OnbidStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.data.OnbidStats(r.Context())
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": stats})
}

// CryptoMarkets serves GET /crypto/markets — 원화 마켓 목록(코인 자산 고르기).
func (h *WealthHandler) CryptoMarkets(w http.ResponseWriter, r *http.Request) {
	board := h.values.CryptoBoard(r.Context())
	list := make([]any, 0, len(board))
	keys := make([]string, 0, len(board))
	for k := range board {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		list = append(list, board[k])
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": list})
}
