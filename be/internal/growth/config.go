/*
Package growth — 검색 유입과 사용자 활성화를 매일 재고 주마다 기회 목록을 뽑는다(worker-growth).

# 무엇을 하고 무엇을 하지 않나 (2026-09-25 사용자 결정)

AI 에이전트가 아니라 **코드만으로** 도는 별도 컨테이너다. LLM을 부르지 않는다. 판단은 전부
규칙(`analyze.go`)이고, 규칙이 못 하는 판단(문안 · 설계)은 보고서에 「사람이 할 일」로 넘긴다.

이 패키지는 앱의 표를 **읽기만** 한다. 쓰는 곳은 growth_* 표와 보고서 파일뿐이다 — 투자
알고리즘 · 점수 산식 · 금융 원본 · 백테스트 · 수익률 · 개인정보 · 인증 · 결제 · 법적 고지를
자동으로 바꾸지 않는다는 약속을 코드 구조로 지킨다(`store_guard_test.go`가 쓰기 문장을 센다).
사용자 표에서는 **개수만** 센다 — 이메일·이름 같은 값은 한 번도 꺼내지 않는다.

코드를 바꾸는 PR도 만들지 않는다. 기술 문제와 콘텐츠 후보는 보고서에 「고칠 자리」와 함께
적고, 브랜치·리뷰·배포는 사람이 한다.

# 외부 소스는 전부 선택

자격 증명이 없는 소스는 건너뛰고 보고서에 「미설정」으로 적는다. 아무것도 없어도 크롤과
DB 지표만으로 돈다. 설정법은 docs/GROWTH.md.
*/
package growth

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config — 전부 환경 변수에서 읽는다. 이 워커 말고는 아무도 쓰지 않는 값이라
// 앱 공용 config에 넣지 않았다.
type Config struct {
	// SiteURL — 사이트맵과 robots.txt를 받는 공개 주소. 끝의 / 없이.
	SiteURL string
	/*
		CrawlOrigin — 비어 있지 않으면 요청을 이 주소로 보내고 Host 헤더만 SiteURL의 것을 쓴다.

		기본은 공개 주소(Cloudflare 앞단 포함)다 — 크롤러가 실제로 보는 것을 재야 하므로.
		Cloudflare가 이 워커를 봇으로 막으면(403 · 챌린지) nginx 컨테이너 주소를 넣는다.
	*/
	CrawlOrigin      string
	CrawlConcurrency int
	CrawlDelay       time.Duration
	UserAgent        string

	// GoogleCredentialsFile — 서비스 계정 JSON 경로. Search Console과 GA4가 함께 쓴다.
	GoogleCredentialsFile string
	// GSCProperty — "sc-domain:quant-port.com"(도메인 속성) 또는 "https://www.quant-port.com/".
	GSCProperty string
	// GA4PropertyID — 숫자 속성 ID(측정 ID G-… 가 아니다).
	GA4PropertyID string
	BingAPIKey    string
	// InspectPerDay — URL 검사 하루 건수. Google 한도는 속성당 하루 2,000 · 분당 600.
	InspectPerDay int

	// ReportDir — 보고서 Markdown 사본을 남길 곳. 비면 DB에만 둔다.
	ReportDir string
}

func LoadConfig() Config {
	return Config{
		SiteURL:               strings.TrimRight(env("GROWTH_SITE_URL", "https://www.quant-port.com"), "/"),
		CrawlOrigin:           strings.TrimRight(env("GROWTH_CRAWL_ORIGIN", ""), "/"),
		CrawlConcurrency:      envInt("GROWTH_CRAWL_CONCURRENCY", 2),
		CrawlDelay:            time.Duration(envInt("GROWTH_CRAWL_DELAY_MS", 500)) * time.Millisecond,
		UserAgent:             env("GROWTH_CRAWL_USER_AGENT", "Mozilla/5.0 (compatible; QuantPortGrowthAudit/1.0; +https://www.quant-port.com)"),
		GoogleCredentialsFile: env("GROWTH_GOOGLE_CREDENTIALS_FILE", ""),
		GSCProperty:           env("GROWTH_GSC_PROPERTY", "sc-domain:quant-port.com"),
		GA4PropertyID:         env("GROWTH_GA4_PROPERTY_ID", ""),
		BingAPIKey:            env("GROWTH_BING_API_KEY", ""),
		InspectPerDay:         envInt("GROWTH_INSPECT_PER_DAY", 200),
		ReportDir:             env("GROWTH_REPORT_DIR", ""),
	}
}

func env(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v, err := strconv.Atoi(strings.TrimSpace(os.Getenv(key))); err == nil && v >= 0 {
		return v
	}
	return def
}

// kst — 보고서의 날짜는 전부 서울 날짜다.
var kst = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Seoul")
	if err != nil {
		return time.FixedZone("KST", 9*3600)
	}
	return loc
}()

func dayOf(t time.Time) time.Time {
	t = t.In(kst)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, kst)
}

func ymd(t time.Time) string { return t.In(kst).Format("2006-01-02") }
