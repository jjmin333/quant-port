package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata"
	"github.com/enact/quant-port/be/internal/repository"
)

/*
FinProductService — 금감원 「금융상품 한눈에」 공시를 하루 한 번 통째로 받는다 (2026-09-29).

상품 5종(정기예금·적금·주택담보·전세자금·개인신용) × 권역 5 × 페이지, 그리고 금융회사
(홈페이지·전화) 권역 5. 1초 간격이라 전부 1~2분이다. 로테이션을 두지 않는 이유는 양이 작고
금리가 자주 바뀌기 때문이다(BATCH_SCHEDULE 원칙 2).

값은 **옮겨 적지 않는다** — 옵션 목록을 상품에 붙이는 것 말고는 API 원문 그대로 저장하고,
계산(세후 이자·월 상환액)은 FE가 그 원문으로 한다.
*/
type FinProductService struct {
	client *marketdata.FinlifeClient
	repo   *repository.FinProductRepo
	// gap — 호출 사이 간격. 버킷 MinInterval(1초)보다 조금 길게 — Check는 기다려 주지 않는다.
	gap time.Duration
}

func NewFinProductService(client *marketdata.FinlifeClient, repo *repository.FinProductRepo) *FinProductService {
	return &FinProductService{client: client, repo: repo, gap: 1100 * time.Millisecond}
}

// finMaxPages — 한 권역에서 따라갈 최대 페이지. 응답의 max_page_no가 이상해도 끝이 있게.
const finMaxPages = 20

func (s *FinProductService) Run(ctx context.Context) (map[string]any, string, error) {
	if s == nil || !s.client.Enabled() {
		return nil, "", fmt.Errorf("fin products: FSS_FINLIFE_API_KEY not configured")
	}
	result := map[string]any{}
	var failures []string
	first := true
	fetch := func(service, group string, page int) (marketdata.FinlifePage, error) {
		if !first {
			select {
			case <-ctx.Done():
				return marketdata.FinlifePage{}, ctx.Err()
			case <-time.After(s.gap):
			}
		}
		first = false
		return s.client.Page(ctx, service, group, page)
	}
	// 한 권역의 모든 페이지.
	sweep := func(service, group string) (marketdata.FinlifePage, error) {
		var all marketdata.FinlifePage
		for page := 1; page <= finMaxPages; page++ {
			p, err := fetch(service, group, page)
			if err != nil {
				return all, err
			}
			all.Base = append(all.Base, p.Base...)
			all.Options = append(all.Options, p.Options...)
			if page >= p.MaxPageNo {
				break
			}
		}
		return all, nil
	}

	// 회사부터 — 상품 화면의 홈페이지 링크가 여기서 온다.
	companies := 0
	for _, group := range marketdata.FinlifeGroups {
		p, err := sweep("companySearch", group)
		if err != nil {
			failures = append(failures, fmt.Sprintf("company %s: %v", group, err))
			continue
		}
		rows := make([]repository.FinCompanyRow, 0, len(p.Base))
		for _, b := range p.Base {
			rows = append(rows, repository.FinCompanyRow{
				FinCoNo: marketdata.FinStr(b, "fin_co_no"), Group: group, Name: marketdata.FinStr(b, "kor_co_nm"),
				HomepageURL: safeHomepage(marketdata.FinStr(b, "homp_url")), Phone: marketdata.FinStr(b, "cal_tel"),
			})
		}
		if err := s.repo.UpsertCompanies(ctx, rows); err != nil {
			return result, "", err
		}
		companies += len(rows)
	}
	result["companies"] = companies

	kinds := make([]string, 0, len(marketdata.FinlifeServices))
	for k := range marketdata.FinlifeServices {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	for _, kind := range kinds {
		// 밀리초로 자른다 — TIMESTAMP(3)에 쓴 값과 비교 값이 정확히 같아야 방금 쓴 행이 안 숨는다.
		at := time.Now().Truncate(time.Millisecond)
		written, ok := 0, true
		for _, group := range marketdata.FinlifeGroups {
			p, err := sweep(marketdata.FinlifeServices[kind], group)
			if err != nil {
				ok = false
				failures = append(failures, fmt.Sprintf("%s %s: %v", kind, group, err))
				continue
			}
			rows, err := buildFinRows(kind, group, p)
			if err != nil {
				return result, "", err
			}
			if err := s.repo.UpsertProducts(ctx, rows, at); err != nil {
				return result, "", err
			}
			written += len(rows)
		}
		entry := map[string]any{"products": written}
		if ok {
			hidden, err := s.repo.DeactivateMissing(ctx, kind, at)
			if err != nil {
				return result, "", err
			}
			entry["hidden"] = hidden
		}
		result[kind] = entry
	}

	msg := fmt.Sprintf("회사 %d", companies)
	for _, kind := range kinds {
		msg += fmt.Sprintf(" · %s %v", kind, result[kind].(map[string]any)["products"])
	}
	if len(failures) > 0 {
		result["failures"] = failures
		// 전부 실패면 오류 — 키가 틀렸거나 API가 죽은 날이 「성공 0건」으로 묻히지 않게.
		if len(failures) == len(marketdata.FinlifeGroups)*(len(kinds)+1) {
			return result, "", fmt.Errorf("fin products: every call failed — %s", failures[0])
		}
		msg += fmt.Sprintf(" · 실패 %d (%s)", len(failures), failures[0])
	}
	return result, msg, nil
}

// buildFinRows attaches each option row to its product (fin_co_no + fin_prdt_cd).
func buildFinRows(kind, group string, p marketdata.FinlifePage) ([]repository.FinProductRow, error) {
	type key struct{ co, cd string }
	opts := map[key][]map[string]any{}
	for _, o := range p.Options {
		k := key{marketdata.FinStr(o, "fin_co_no"), marketdata.FinStr(o, "fin_prdt_cd")}
		// 상품에 이미 있는 칸은 뺀다 — 옵션 수만큼 반복돼 응답만 키운다.
		clean := make(map[string]any, len(o))
		for f, v := range o {
			if f != "fin_co_no" && f != "fin_prdt_cd" && f != "dcls_month" {
				clean[f] = v
			}
		}
		opts[k] = append(opts[k], clean)
	}
	rows := make([]repository.FinProductRow, 0, len(p.Base))
	for _, b := range p.Base {
		k := key{marketdata.FinStr(b, "fin_co_no"), marketdata.FinStr(b, "fin_prdt_cd")}
		if k.co == "" || k.cd == "" {
			continue
		}
		base, err := json.Marshal(b)
		if err != nil {
			return nil, err
		}
		o := opts[k]
		if o == nil {
			o = []map[string]any{}
		}
		options, err := json.Marshal(o)
		if err != nil {
			return nil, err
		}
		rows = append(rows, repository.FinProductRow{
			Kind: kind, FinCoNo: k.co, ProductCode: k.cd, Group: group,
			CompanyName: marketdata.FinStr(b, "kor_co_nm"), ProductName: marketdata.FinStr(b, "fin_prdt_nm"),
			DclsMonth: marketdata.FinStr(b, "dcls_month"), DclsStartDay: marketdata.FinStr(b, "dcls_strt_day"),
			DclsEndDay: marketdata.FinStr(b, "dcls_end_day"), Base: base, Options: options,
		})
	}
	return rows, nil
}

// safeHomepage keeps only http(s) links — 화면이 그대로 href에 넣는다(javascript: 등 차단).
// 공시에는 「www.xxx.co.kr」처럼 스킴 없는 값도 있어 https를 붙인다.
func safeHomepage(u string) string {
	u = strings.TrimSpace(u)
	if u == "" {
		return ""
	}
	lower := strings.ToLower(u)
	if strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://") {
		return u
	}
	if strings.Contains(lower, ":") || strings.ContainsAny(u, " <>\"'") {
		return ""
	}
	return "https://" + u
}
