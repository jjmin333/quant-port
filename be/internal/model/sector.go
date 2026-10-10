package model

import (
	"strconv"
	"strings"
)

/*
Sector — 국내·미국 종목을 한 줄에 세우는 **공통 섹터 11개**.

# 왜 필요한가

점수의 피어그룹이 지금은 (자산유형 × 시장 × 시총 3분위)다. 섹터가 없어서다
(`docs/STOCK_SCORE_DESIGN.md` §2.5). PER 40은 반도체에서 평범하고 은행에서
비정상인데, 한 줄로 세우면 가치 축은 사실상 「업종 판별기」가 된다.

# 왜 GICS를 그대로 쓰지 않나

GICS는 라이선스 데이터다. 우리가 가진 것은 공시 원천이 주는 **산업 코드**뿐이다 —
국내는 DART `induty_code`(KSIC, 한국표준산업분류), 미국은 SEC `sic`(SIC 4자리).
둘은 체계가 다르므로 GICS 11섹터와 **이름과 경계가 비슷한** 공통 칸으로 옮긴다.
옮기는 규칙은 근사다 — 예를 들어 포털(네이버·카카오)은 커뮤니케이션으로 보내고,
지주회사는 이름으로 금융지주만 가려내고 나머지는 비워 둔다(SectorFromKSICNamed). 경계가 애매한 코드는 아래 함수의
주석이 이유를 적는다.

모르는 코드는 빈 값(`SectorUnknown`)이다. 짐작해서 칸에 넣지 않는다 — 틀린 칸에
들어간 종목은 그 칸의 백분위를 조용히 흔든다.
*/
type Sector string

const (
	SectorUnknown        Sector = ""
	SectorEnergy         Sector = "energy"
	SectorMaterials      Sector = "materials"
	SectorIndustrials    Sector = "industrials"
	SectorConsumerDisc   Sector = "consumer_discretionary"
	SectorConsumerStaple Sector = "consumer_staples"
	SectorHealthCare     Sector = "health_care"
	SectorFinancials     Sector = "financials"
	SectorIT             Sector = "information_technology"
	SectorCommunication  Sector = "communication"
	SectorUtilities      Sector = "utilities"
	SectorRealEstate     Sector = "real_estate"
)

// Sectors is the display order.
var Sectors = []Sector{
	SectorIT, SectorCommunication, SectorConsumerDisc, SectorConsumerStaple,
	SectorHealthCare, SectorFinancials, SectorIndustrials, SectorMaterials,
	SectorEnergy, SectorUtilities, SectorRealEstate,
}

var sectorLabels = map[Sector]string{
	SectorEnergy:         "에너지",
	SectorMaterials:      "소재",
	SectorIndustrials:    "산업재",
	SectorConsumerDisc:   "경기소비재",
	SectorConsumerStaple: "필수소비재",
	SectorHealthCare:     "헬스케어",
	SectorFinancials:     "금융",
	SectorIT:             "IT",
	SectorCommunication:  "커뮤니케이션",
	SectorUtilities:      "유틸리티",
	SectorRealEstate:     "부동산",
}

// Label is the Korean display name ("" for unknown).
func (s Sector) Label() string { return sectorLabels[s] }

// AllSectors lists the 11 common sectors in a fixed order (문장 검색 사전 후보용).
func AllSectors() []Sector {
	return []Sector{SectorEnergy, SectorMaterials, SectorIndustrials, SectorConsumerDisc, SectorConsumerStaple,
		SectorHealthCare, SectorFinancials, SectorIT, SectorCommunication, SectorUtilities, SectorRealEstate}
}

// Valid reports whether s is one of the eleven sectors.
func (s Sector) Valid() bool { _, ok := sectorLabels[s]; return ok }

/*
SectorFromKSIC — DART `induty_code`(KSIC 10차, 2~5자리)를 섹터로.

DART는 코드만 준다(이름 없음). 앞 두 자리(중분류)로 대부분이 갈리고, 한 중분류
안에서 섹터가 갈리는 곳만 더 긴 접두어로 가른다. **긴 접두어를 먼저 본다.**
*/
func SectorFromKSIC(code string) Sector {
	code = strings.TrimSpace(code)
	if len(code) < 2 {
		return SectorUnknown
	}
	for _, o := range ksicOverrides {
		if strings.HasPrefix(code, o.prefix) {
			return o.sector
		}
	}
	div, err := strconv.Atoi(code[:2])
	if err != nil {
		return SectorUnknown
	}
	switch {
	case div >= 1 && div <= 3: // 농업·임업·어업
		return SectorConsumerStaple
	case div == 5 || div == 8: // 석탄 · 광업 지원 서비스
		return SectorEnergy
	case div == 6 || div == 7: // 금속·비금속 광물
		return SectorMaterials
	case div >= 10 && div <= 12: // 식료품·음료·담배
		return SectorConsumerStaple
	case div >= 13 && div <= 15: // 섬유·의복·가죽
		return SectorConsumerDisc
	case div == 16 || div == 17: // 목재·펄프·종이
		return SectorMaterials
	case div == 18: // 인쇄
		return SectorIndustrials
	case div == 19: // 코크스·석유정제
		return SectorEnergy
	case div == 20: // 화학(의약품 제외) — 화장품은 override
		return SectorMaterials
	case div == 21: // 의약품
		return SectorHealthCare
	case div >= 22 && div <= 24: // 고무·플라스틱 · 비금속광물제품 · 1차 금속
		return SectorMaterials
	case div == 25: // 금속가공제품
		return SectorIndustrials
	case div == 26: // 전자부품·컴퓨터·통신장비 — 반도체가 여기다
		return SectorIT
	case div == 27: // 의료·정밀·광학 — 의료기기는 override
		return SectorIT
	case div == 28 || div == 29: // 전기장비(배터리 포함) · 기타 기계
		return SectorIndustrials
	case div == 30: // 자동차
		return SectorConsumerDisc
	case div == 31: // 선박·항공기 등
		return SectorIndustrials
	case div == 32 || div == 33: // 가구 · 기타 제품
		return SectorConsumerDisc
	case div == 35 || div == 36: // 전기·가스 · 수도
		return SectorUtilities
	case div >= 37 && div <= 39: // 하수·폐기물·환경 복원
		return SectorIndustrials
	case div == 41 || div == 42: // 건설
		return SectorIndustrials
	case div == 45: // 자동차 판매
		return SectorConsumerDisc
	case div == 46: // 도매
		return SectorIndustrials
	case div == 47: // 소매 — 음식료품 소매는 override
		return SectorConsumerDisc
	case div >= 49 && div <= 52: // 운송·창고
		return SectorIndustrials
	case div == 55 || div == 56: // 숙박·음식점
		return SectorConsumerDisc
	case div == 58: // 출판 — 소프트웨어 출판은 override
		return SectorCommunication
	case div >= 59 && div <= 61: // 영상·방송·통신
		return SectorCommunication
	case div == 62: // 컴퓨터 프로그래밍·시스템 통합
		return SectorIT
	case div == 63: // 정보서비스(포털·데이터 처리)
		return SectorCommunication
	case div >= 64 && div <= 66: // 금융·보험 — 지주회사 코드는 SectorFromKSICNamed가 가른다
		return SectorFinancials
	case div == 68: // 부동산
		return SectorRealEstate
	case div == 70: // 연구개발 — 의약학 연구는 override
		return SectorIndustrials
	case div >= 71 && div <= 76: // 전문·과학·사업지원·임대 — 광고는 override
		return SectorIndustrials
	case div == 85: // 교육
		return SectorConsumerDisc
	case div == 86 || div == 87: // 보건·사회복지
		return SectorHealthCare
	case div == 90 || div == 91: // 예술·스포츠·여가(카지노 포함)
		return SectorConsumerDisc
	case div >= 94 && div <= 96: // 협회·수리·개인 서비스
		return SectorConsumerDisc
	}
	return SectorUnknown
}

type sectorPrefix struct {
	prefix string
	sector Sector
}

// ksicOverrides — 한 중분류 안에서 섹터가 갈리는 코드. 긴 것부터 둔다.
var ksicOverrides = []sectorPrefix{
	// 화장품(20423) · 비누·세제(20421)는 필수소비재 — 아모레퍼시픽·LG생활건강.
	{"2042", SectorConsumerStaple},
	// 가정용 기기(285) — 가전은 경기소비재.
	{"285", SectorConsumerDisc},
	// 의료용 기기(271).
	{"271", SectorHealthCare},
	// 음식료품·담배 소매(472).
	{"472", SectorConsumerStaple},
	// 소프트웨어 출판(582) — 게임·패키지 SW.
	{"582", SectorIT},
	// 의학·약학 연구개발(70113 등 7011x) — 바이오 기업의 대부분이 이 코드로 등록한다.
	{"7011", SectorHealthCare},
	// 광고(713).
	{"713", SectorCommunication},
}

/*
SectorFromKSICNamed — 지주회사 코드만 회사명으로 한 번 더 가른다.

**KSIC로는 지주회사의 업종을 알 수 없다** (2026-09-23 실측). 금융지주(신한지주 ·
iM금융지주)와 비금융 지주(삼양홀딩스 · 한국앤컴퍼니)가 같은 `64992`이고, 반대로
HDC · 오리온홀딩스 · 효성은 `649`(기타 금융업)로 등록돼 있다. 코드만 보면 신한지주가
산업재가 되고 효성이 금융이 된다.

그래서 지주회사 코드에서는 이름이 금융 성격을 밝히는 경우(금융 · 스팩 · 캐피탈 ·
인베스트 · 투자)만 금융으로 보내고, **나머지는 비워 둔다.** 비금융 지주의 실제 업종은
주력 자회사가 정하는데 우리에겐 그 정보가 없다 — 산업재로 짐작해 넣으면 그 칸의
백분위를 조용히 흔든다.
*/
func SectorFromKSICNamed(code, name string) Sector {
	if !isKSICHoldingCompany(code) {
		return SectorFromKSIC(code)
	}
	n := strings.ReplaceAll(name, " ", "")
	if _, ok := krFinancialHoldingNames[n]; ok {
		return SectorFinancials
	}
	for _, kw := range []string{"금융", "스팩", "캐피탈", "인베스트", "투자"} {
		if strings.Contains(n, kw) {
			return SectorFinancials
		}
	}
	return SectorUnknown
}

// isKSICHoldingCompany — 지주회사가 등록되는 코드: 649(기타 금융업 · 3자리로만
// 주는 경우)와 6499x(그 외 기타 금융업 — 64992 지주회사 포함).
func isKSICHoldingCompany(code string) bool {
	code = strings.TrimSpace(code)
	return code == "649" || strings.HasPrefix(code, "6499")
}

// krFinancialHoldingNames — 이름에 금융 성격이 드러나지 않는 금융지주. 짧게 유지한다.
var krFinancialHoldingNames = map[string]struct{}{
	"신한지주": {},
}

/*
SectorFromSIC — SEC `sic`(4자리)를 섹터로.

SIC는 1987년 체계라 소프트웨어·인터넷의 칸이 거칠다. 7370~7379(컴퓨터 서비스)를
IT로, 인터넷 서비스가 흔히 쓰는 7370도 같이 IT로 둔다 — GICS는 알파벳·메타를
커뮤니케이션에 두지만, SIC 코드만으로는 소프트웨어 회사와 가를 수 없다. 틀리는
방향을 하나로 정한 것이다.
*/
func SectorFromSIC(sic string) Sector {
	n, err := strconv.Atoi(strings.TrimSpace(sic))
	if err != nil || n <= 0 {
		return SectorUnknown
	}
	switch {
	case n < 1000:
		return SectorConsumerStaple // 농림어업
	case n < 1100:
		return SectorMaterials // 금속 광업
	case n >= 1200 && n < 1400:
		return SectorEnergy // 석탄 · 석유·가스 채굴
	case n >= 1400 && n < 1500:
		return SectorMaterials // 비금속 광물
	case n >= 1500 && n < 1800:
		return SectorIndustrials // 건설
	case n >= 2000 && n < 2200:
		return SectorConsumerStaple // 식품·담배
	case n >= 2200 && n < 2400:
		return SectorConsumerDisc // 섬유·의류
	case n >= 2400 && n < 2500:
		return SectorMaterials // 목재
	case n >= 2500 && n < 2600:
		return SectorConsumerDisc // 가구
	case n >= 2600 && n < 2700:
		return SectorMaterials // 종이
	case n >= 2700 && n < 2800:
		return SectorCommunication // 인쇄·출판
	case n >= 2830 && n <= 2836:
		return SectorHealthCare // 의약품·생물학적 제제
	case n >= 2840 && n <= 2844:
		return SectorConsumerStaple // 비누·세제·화장품
	case n >= 2800 && n < 2900:
		return SectorMaterials // 화학
	case n >= 2900 && n < 3000:
		return SectorEnergy // 석유 정제
	case n >= 3000 && n < 3100:
		return SectorMaterials // 고무·플라스틱
	case n >= 3100 && n < 3200:
		return SectorConsumerDisc // 가죽
	case n >= 3200 && n < 3400:
		return SectorMaterials // 석재·유리 · 1차 금속
	case n >= 3400 && n < 3500:
		return SectorIndustrials // 금속가공
	case n >= 3570 && n < 3580:
		return SectorIT // 컴퓨터·사무기기
	case n >= 3500 && n < 3600:
		return SectorIndustrials // 산업 기계
	case n >= 3630 && n < 3640, n == 3651, n == 3652:
		return SectorConsumerDisc // 가전 · 가정용 음향
	case n >= 3660 && n < 3680:
		return SectorIT // 통신장비 · 반도체·전자부품
	case n >= 3600 && n < 3700:
		return SectorIndustrials // 그 밖의 전기 장비
	case n >= 3710 && n < 3720, n >= 3750 && n < 3760:
		return SectorConsumerDisc // 자동차 · 이륜차
	case n >= 3700 && n < 3800:
		return SectorIndustrials // 항공·선박·철도·방산
	case n >= 3840 && n < 3860:
		return SectorHealthCare // 의료기기·안경
	case n >= 3800 && n < 3900:
		return SectorIT // 계측·광학
	case n >= 3900 && n < 4000:
		return SectorConsumerDisc // 보석·완구·기타 제조
	case n >= 4000 && n < 4800:
		return SectorIndustrials // 운송
	case n >= 4800 && n < 4900:
		return SectorCommunication // 통신·방송
	case n >= 4950 && n < 4960:
		return SectorIndustrials // 폐기물 처리
	case n >= 4900 && n < 5000:
		return SectorUtilities // 전기·가스·수도
	case n == 5122:
		return SectorHealthCare // 의약품 도매
	case n >= 5140 && n < 5150:
		return SectorConsumerStaple // 식료품 도매
	case n >= 5000 && n < 5200:
		return SectorIndustrials // 도매
	case n >= 5400 && n < 5500, n == 5912:
		return SectorConsumerStaple // 식료품점 · 약국
	case n >= 5200 && n < 6000:
		return SectorConsumerDisc // 소매·외식
	case n == 6798:
		return SectorRealEstate // 리츠
	case n >= 6500 && n < 6600:
		return SectorRealEstate // 부동산
	case n >= 6000 && n < 6800:
		return SectorFinancials // 금융·보험·투자회사
	case n >= 7000 && n < 7100:
		return SectorConsumerDisc // 숙박
	case n >= 7200 && n < 7300:
		return SectorConsumerDisc // 개인 서비스
	case n >= 7310 && n < 7320:
		return SectorCommunication // 광고
	case n >= 7370 && n < 7380:
		return SectorIT // 소프트웨어·컴퓨터 서비스
	case n >= 7300 && n < 7400:
		return SectorIndustrials // 사업 서비스
	case n >= 7500 && n < 7600:
		return SectorConsumerDisc // 자동차 정비
	case n >= 7800 && n < 7900:
		return SectorCommunication // 영화
	case n >= 7900 && n < 8000:
		return SectorConsumerDisc // 오락·여가
	case n >= 8000 && n < 8100:
		return SectorHealthCare // 보건 서비스
	case n == 8731:
		return SectorHealthCare // 상업적 물리·생물 연구 — 바이오
	case n >= 8200 && n < 8300:
		return SectorConsumerDisc // 교육
	case n >= 8100 && n < 9000:
		return SectorIndustrials // 법률·엔지니어링·경영 서비스
	}
	return SectorUnknown // 9100~9999: 공공·분류 불능(SPAC 6770은 위에서 금융)
}
