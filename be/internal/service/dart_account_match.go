package service

import "strings"

/*
DART 계정명 매칭.

**왜 정규화가 필요한가.** DART는 제출인이 쓴 계정명을 그대로 돌려준다. 같은 개념이
회사마다, 심지어 **같은 회사의 분기마다** 다른 이름으로 온다. 실측 두 건:

  - SK하이닉스 2025 연간: EPS가 `기본주당순이익(손실)`
  - SK하이닉스 2024 1Q:  매출이 `영업수익` (다른 분기는 전부 `매출액`)

예전 매칭은 「완전 일치 또는 이름+"(손실)"」뿐이라 둘 다 놓쳤고, 그래서 차트 **중간에**
구멍이 났다. 규모: 분기 행 기준 DART 매출 21.3% · EPS 32.2%가 비어 있었고,
전 분기가 빈 종목이 매출 235개 · EPS 242개였다.

**어떻게 고쳤나.** 세 단계다.

 1. 정규화 — 공백 제거, 괄호 묶음 제거(`(손실)`·`(매출액)`), 기간 표지 제거
    (`당기순`·`분기순`·`반기순` → `순`). `기본주당분기순이익(손실)` → `기본주당순이익`.
 2. 재무제표 구분(`sj_div`)으로 후보를 좁힌다 — 손익 항목은 손익계산서에서만,
    잔액 항목은 재무상태표에서만 찾는다. `매출채권`(BS)이 매출로 잡히는 부류의
    사고를 원천 차단한다. 응답에 구분이 없으면 거르지 않는다(예전 동작 유지).
 3. **계층 매칭** — 1순위 이름들에서 하나라도 찾으면 거기서 끝낸다. 못 찾을 때만
    2순위로 내려간다. 「지배기업 소유주 귀속 순이익」처럼 개념이 살짝 다른 이름을
    총액과 섞지 않으려는 것이다.

접미사·접두사 규칙을 쓰지 않고 **정규화된 완전 일치**를 고수한다. `수익`을 부분
일치로 잡으면 `금융수익`·`기타영업외수익`이 매출이 되고, 그건 빈칸보다 나쁘다.
*/

// dartStatement — DART `sj_div` 값. 손익 항목은 IS/CIS, 잔액 항목은 BS에 있다.
const (
	dartStmtBS  = "BS"  // 재무상태표
	dartStmtIS  = "IS"  // 손익계산서
	dartStmtCIS = "CIS" // 포괄손익계산서
)

// dartAccountRule은 한 필드를 찾는 방법이다.
type dartAccountRule struct {
	// Statements가 비어 있지 않으면 그 재무제표의 행만 본다.
	Statements []string
	// Tiers는 우선순위 순 정규화 이름 묶음. 앞선 묶음에서 찾으면 뒤는 안 본다.
	Tiers [][]string
	// AllowOwnerPrefix면 모든 계층이 빈 뒤에 「지배기업 소유주지분」류 접두사를
	// 떼고 1순위를 한 번 더 본다. EPS 전용이다 — IFRS의 주당이익은 원래
	// 「지배기업 소유주 귀속」 기준이라 접두사를 붙여 쓰는 제출인이 있을 뿐,
	// 개념이 다른 값이 아니다(실측: 제일파마홀딩스 `지배기업 소유주지분 기본주당손익`).
	//
	// 순이익에는 켜지 않는다 — 거기서는 총액과 귀속분이 실제로 다른 값이고,
	// 이미 2순위로 따로 다룬다.
	AllowOwnerPrefix bool
}

// dartOwnerPrefixes — 「누구에게 귀속되는지」를 앞에 적는 관용구. 정규화된 형태다.
var dartOwnerPrefixes = []string{
	"지배기업의소유주에게귀속되는",
	"지배기업소유주에게귀속되는",
	"지배기업소유주지분",
	"지배기업소유주귀속",
	"지배기업지분",
	"지배주주지분",
	"지배주주",
}

// stripOwnerPrefix는 귀속 접두사를 하나 떼어 낸다. 없으면 그대로 돌려준다.
//
// **접미사·부분 일치로 확장하지 않는다.** `우선주기본주당순이익`은 여기서 걸리지
// 않아야 한다 — 보통주가 아닌 다른 주식 종류의 EPS라 총액 자리에 놓으면 틀린
// 값이 된다(실측: 대상 001680은 연결재무제표에 우선주 EPS만 있다).
func stripOwnerPrefix(normalized string) string {
	for _, p := range dartOwnerPrefixes {
		if strings.HasPrefix(normalized, p) {
			return strings.TrimPrefix(normalized, p)
		}
	}
	return normalized
}

// dartAccountRules — 필드별 계정명 규칙.
//
// 이름은 **정규화된 형태**로 적는다(괄호·공백·기간 표지가 이미 제거된 모습).
// 예: `수익(매출액)` → `수익`, `당기순이익` → `순이익`.
var dartAccountRules = map[string]dartAccountRule{
	"revenue": {
		Statements: []string{dartStmtIS, dartStmtCIS},
		Tiers: [][]string{
			// 1순위: 매출 그 자체를 뜻하는 이름들.
			{"매출액", "매출", "수익", "영업수익", "매출및지분법손익", "총매출액", "영업수익매출액"},
			// 2순위: 순매출만 적는 제출인.
			{"순매출액", "매출수익"},
		},
	},
	"operating_income": {
		Statements: []string{dartStmtIS, dartStmtCIS},
		Tiers: [][]string{
			{"영업이익", "영업손익", "영업이익손실"},
		},
	},
	"net_income": {
		Statements: []string{dartStmtIS, dartStmtCIS},
		Tiers: [][]string{
			// `당기순이익`·`분기순이익`·`반기순이익`이 모두 여기로 정규화된다.
			{"순이익", "순손익", "순이익손실", "총순이익"},
			// 2순위: 총액을 안 적고 귀속분만 적는 제출인. 개념이 다르므로
			// 1순위가 비었을 때만 쓴다.
			{
				"지배기업의소유주에게귀속되는순이익",
				"지배기업소유주지분에귀속되는순이익",
				"지배기업소유주귀속순이익",
				"지배주주순이익",
			},
		},
	},
	"total_assets":        {Statements: []string{dartStmtBS}, Tiers: [][]string{{"자산총계", "총자산"}}},
	"total_equity":        {Statements: []string{dartStmtBS}, Tiers: [][]string{{"자본총계", "총자본"}}},
	"total_liabilities":   {Statements: []string{dartStmtBS}, Tiers: [][]string{{"부채총계", "총부채"}}},
	"current_assets":      {Statements: []string{dartStmtBS}, Tiers: [][]string{{"유동자산", "유동자산계"}}},
	"current_liabilities": {Statements: []string{dartStmtBS}, Tiers: [][]string{{"유동부채", "유동부채계"}}},
	"eps_basic": {
		Statements:       []string{dartStmtIS, dartStmtCIS},
		AllowOwnerPrefix: true,
		Tiers: [][]string{
			// `기본주당순이익(손실)`·`기본주당분기순이익(손실)`이 여기로 온다.
			{"기본주당이익", "기본주당순이익", "기본주당손익", "기본주당순손익", "기본및희석주당이익", "기본및희석주당순이익"},
			// 2순위: `기본`을 안 붙이는 제출인.
			{"주당이익", "주당순이익", "주당손익", "주당순손익"},
		},
	},
	"eps_diluted": {
		Statements:       []string{dartStmtIS, dartStmtCIS},
		AllowOwnerPrefix: true,
		Tiers: [][]string{
			{"희석주당이익", "희석주당순이익", "희석주당손익", "희석주당순손익", "기본및희석주당이익", "기본및희석주당순이익"},
		},
	},
}

// dartPeriodMarkers — 계정명에 끼어드는 기간 표지. 개념이 아니라 「어느 기간의
// 것인지」를 말할 뿐이라 지운다. `당기순이익`/`분기순이익`/`반기순이익`이 모두
// `순이익` 하나로 모인다.
//
// **`순` 앞에서만 지운다.** 예전에는 `분기`를 통째로 `ReplaceAll` 했는데, 그러면
// 낱말 경계를 넘어 자른다 — `지배기업소유주지분기본주당손익`의 「지**분기**본」이
// 걸려 `지배기업소유주지본주당손익`이 됐다. 표지는 언제나 `순이익`·`순손익` 바로
// 앞에 붙으므로 그 자리에서만 떼면 안전하다.
//
// `전기`는 넣지 않는다 — 그 열은 비교표시용 과거 값이고, 계정명에 붙어 오는 일도
// 없다. 넣으면 `전기료` 같은 무관한 이름을 건드린다.
var dartPeriodMarkers = []string{"당기순", "분기순", "반기순"}

// dartLeadingMarkers — 이름 맨 앞에만 붙는 표지. 가운데에서 지우면 안 된다.
var dartLeadingMarkers = []string{"연결", "누적"}

// normalizeDartAccountName은 비교용 형태로 줄인다.
//
// 괄호를 통째로 지우는 것이 핵심이다. DART는 그 안에 「(손실)」처럼 그 보고서에서만
// 유효한 부호 정보를 넣고, 「수익(매출액)」처럼 동의어를 넣기도 한다. 어느 쪽이든
// 계정을 식별하는 정보는 아니다.
func normalizeDartAccountName(name string) string {
	var b strings.Builder
	depth := 0
	for _, r := range name {
		switch r {
		case '(', '（', '[', '<':
			depth++
		case ')', '）', ']', '>':
			if depth > 0 {
				depth--
			}
		case ' ', '\t', '\n', '\r', ' ', '　', ',', '.', '·', '/', '-', '_':
			// 구분자는 버린다.
		default:
			if depth == 0 {
				b.WriteRune(r)
			}
		}
	}
	out := b.String()
	for _, marker := range dartLeadingMarkers {
		out = strings.TrimPrefix(out, marker)
	}
	for _, marker := range dartPeriodMarkers {
		out = strings.ReplaceAll(out, marker, "순")
	}
	return out
}

// dartRowInStatements는 행이 허용된 재무제표에 속하는지 본다.
// 응답이 구분을 안 주면(옛 픽스처 포함) 거르지 않는다.
func dartRowInStatements(row dartAccountRow, statements []string) bool {
	if len(statements) == 0 {
		return true
	}
	div := strings.ToUpper(strings.TrimSpace(row.StatementDiv))
	if div == "" {
		return true
	}
	for _, s := range statements {
		if div == s {
			return true
		}
	}
	return false
}

// pickDartTiered는 규칙을 순위대로 적용해 첫 성공 계층의 값을 낸다.
// amountOf는 어느 금액 열을 읽을지 정한다(당기 or 당기누적).
func pickDartTiered(rows []dartAccountRow, field string, amountOf func(dartAccountRow) string) *float64 {
	rule, ok := dartAccountRules[field]
	if !ok {
		return nil
	}
	for _, tier := range rule.Tiers {
		if v := pickDartTierMatch(rows, rule, tier, amountOf, false); v != nil {
			return v
		}
	}
	// 마지막 한 번 — 「지배기업 소유주지분 기본주당손익」처럼 귀속 접두사를 붙여
	// 쓴 이름을 1순위로 되돌린다. 계층을 다 소진한 뒤에만 하므로, 접두사 없는
	// 이름이 있으면 그쪽이 언제나 이긴다.
	if rule.AllowOwnerPrefix && len(rule.Tiers) > 0 {
		if v := pickDartTierMatch(rows, rule, rule.Tiers[0], amountOf, true); v != nil {
			return v
		}
	}
	return nil
}

func pickDartTierMatch(
	rows []dartAccountRow,
	rule dartAccountRule,
	tier []string,
	amountOf func(dartAccountRow) string,
	stripPrefix bool,
) *float64 {
	want := make(map[string]struct{}, len(tier))
	for _, n := range tier {
		want[n] = struct{}{}
	}
	var best *float64
	var bestAbs float64
	for _, row := range rows {
		if !dartRowInStatements(row, rule.Statements) {
			continue
		}
		name := normalizeDartAccountName(row.AccountName)
		if stripPrefix {
			stripped := stripOwnerPrefix(name)
			// 실제로 접두사가 떨어진 이름만 본다 — 안 그러면 1순위를 그대로
			// 한 번 더 도는 셈이고, 이미 실패한 검사다.
			if stripped == name {
				continue
			}
			name = stripped
		}
		if _, hit := want[name]; !hit {
			continue
		}
		raw := strings.TrimSpace(amountOf(row))
		if raw == "" {
			continue
		}
		val := parseDartAmount(raw)
		abs := val
		if abs < 0 {
			abs = -abs
		}
		// 같은 계정이 여러 번 오면(연결/별도 열이 섞이는 보고서가 있다)
		// 절댓값이 큰 쪽을 쓴다 — 예전 동작 그대로다.
		if best == nil || abs > bestAbs {
			v := val
			best = &v
			bestAbs = abs
		}
	}
	return best
}

func dartAmountCurrent(row dartAccountRow) string { return row.Amount }
func dartAmountAdd(row dartAccountRow) string     { return row.AddAmount }
