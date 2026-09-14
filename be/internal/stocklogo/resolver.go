package stocklogo

import (
	"strings"

	"github.com/enact/quant-port/be/internal/model"
)

/*
LogoURL은 **내려받아 둔 파일이 있으면 그것**을, 없으면 FMP 원격 URL을 돌려준다.

예전에는 `entry.Source == "naver"`인 항목만 로컬 파일로 쳤다. 그런데 `source`는
"그림을 어디서 받아왔나"이지 "저장해 두었나"가 아니다. 해외 로고는 FMP에서 받아
`/logos/us/*.png`로 저장해 두어도 `source`가 그대로 `"fmp"`라, 이 조건은 **저장된
11,205개 파일을 전부 못 쓰고 매번 원격 CDN을 다시 부르게** 만들었다. 화면 쪽은
같은 이유로 이미 `path` 기준으로 고쳐져 있었는데(`fe/src/lib/stock-logo.ts`),
BE가 내려보낸 `logoUrl`이 화면의 판단을 덮어써서 그 수정이 무력화돼 있었다.

판정 기준은 `path`가 있는지 하나다 — 국내·해외를 가르지 않는다.

`tone == "blank"`는 **빈 문자열**을 돌려준다. 불투명한 흰 이미지라 어두운 판을
깔아도 안 보이고 원격 URL도 같은 그림이므로, 쓸 수 있는 로고가 없는 것으로 친다
(화면은 종목명 두 글자 모노그램을 그린다). tone.go 참고.
*/
func LogoURL(symbol string, currency model.QuoteCurrency, assetType model.QuoteAssetType) string {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if sym == "" {
		return ""
	}

	if m := GetManifest(); m != nil {
		if entry, ok := m.Entries[sym]; ok {
			if entry.Tone == "blank" {
				return ""
			}
			// Windows 예약 이름(CON·PRN…)은 저장소에 파일로 못 들어간다 —
			// 배포본에 없는 파일을 가리키지 않도록 원격 URL로 보낸다(client_index.go).
			if entry.Path != "" && !IsWindowsReservedName(sym) {
				return entry.Path
			}
		}
	}
	return FMPImageURL(symbol, currency, assetType)
}

// FMPFallbackURL returns the FMP CDN URL for img onError fallback (empty when primary is already FMP).
//
// 로고가 없는 것으로 판정된 종목(tone == "blank")에는 폴백도 주지 않는다 — 원격에
// 있는 그림이 화면에서 안 보이던 바로 그 그림이다.
func FMPFallbackURL(symbol string, currency model.QuoteCurrency, assetType model.QuoteAssetType) string {
	primary := LogoURL(symbol, currency, assetType)
	if primary == "" {
		return ""
	}
	fmp := FMPImageURL(symbol, currency, assetType)
	if primary != fmp {
		return fmp
	}
	return ""
}
