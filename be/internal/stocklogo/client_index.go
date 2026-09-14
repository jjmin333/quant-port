package stocklogo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

/*
화면이 읽는 **작은** 매니페스트.

`manifest.json`은 종목마다 객체 하나(region·source·path·reason·tone)를 담아 17,915
항목에서 **2.0 MB**가 된다. 그런데 그 파일을 `stock-logo.ts`가 `import`하므로 전량이
클라이언트 번들에 실려 브라우저가 매 방문마다 2 MB를 파싱했다 — 로고를 빨리 띄우려고
파일을 내려받아 둔 것인데, 그 목록 자체가 첫 화면을 늦추고 있었다.

화면이 실제로 쓰는 사실은 종목당 셋뿐이다: **국내 파일이 있나 · 해외 파일이 있나 ·
톤은 무엇인가**. `path`는 그 둘에서 `/logos/{kr,us}/{심볼}.png`로 되짚을 수 있고
`region`·`source`·`reason`은 만들 때 쓰는 기록이지 화면이 볼 것이 아니다.

그래서 심볼을 쉼표로 이어 붙인 네 문자열만 남긴다 — **86 KB(gzip 39 KB)**로 raw
23배, gzip 3배가 준다. 파싱도 객체 17,915개가 아니라 문자열 넷 쪼개기다.

`blank`은 파일이 있어도 여기서는 「로고 없음」이다 — 불투명한 흰 이미지라 화면에
얹으면 아무것도 안 보인다(tone.go). 그래서 kr·us 목록에서 빼고 blank에만 적는다.

**manifest.json을 쓸 때마다 함께 쓴다**(`SaveManifest`). 두 파일이 어긋나면 화면이
없는 파일을 가리키게 되는데, 손으로 맞추게 두면 반드시 어긋난다.
*/

const clientIndexFileName = "logo-index.json"

// ClientIndex is the browser-facing shape: four comma-joined symbol lists.
type ClientIndex struct {
	// KR / US: 그 폴더에 파일이 있는 심볼. 경로는 화면이 되짚는다.
	KR string `json:"kr"`
	US string `json:"us"`
	// Light: 투명 배경 위의 흰 그림 — 화면이 어두운 판을 깐다.
	Light string `json:"light"`
	// Blank: 판을 깔아도 안 보인다 — 로고가 없는 것으로 친다.
	Blank string `json:"blank"`
}

// BuildClientIndex reduces a full manifest to the four lists the browser needs.
func BuildClientIndex(m *Manifest) ClientIndex {
	if m == nil {
		return ClientIndex{}
	}
	var kr, us, light, blank []string
	for sym, entry := range m.Entries {
		sym = strings.ToUpper(strings.TrimSpace(sym))
		if sym == "" {
			continue
		}
		if entry.Tone == "blank" {
			blank = append(blank, sym)
			continue
		}
		// Windows에서 파일 이름이 될 수 없는 심볼은 저장소에 못 들어간다 —
		// 목록에 적으면 화면이 없는 파일을 가리킨다.
		if IsWindowsReservedName(sym) {
			continue
		}
		switch {
		case strings.HasPrefix(entry.Path, defaultLocalLogoPrefix):
			kr = append(kr, sym)
		case strings.HasPrefix(entry.Path, defaultLocalUSLogoPrefix):
			us = append(us, sym)
		default:
			// 받아 두지 못한 종목. 화면은 원격 URL로 떨어진다 — 적을 것이 없다.
			continue
		}
		if entry.Tone == "light" {
			light = append(light, sym)
		}
	}
	// 정렬은 diff를 읽을 수 있게 하려는 것이다 — 맵 순회 순서 그대로 쓰면 내용이
	// 안 바뀐 실행에서도 파일 전체가 바뀐 것으로 보인다.
	sort.Strings(kr)
	sort.Strings(us)
	sort.Strings(light)
	sort.Strings(blank)
	return ClientIndex{
		KR:    strings.Join(kr, ","),
		US:    strings.Join(us, ","),
		Light: strings.Join(light, ","),
		Blank: strings.Join(blank, ","),
	}
}

// SaveClientIndex writes logo-index.json beside the given manifest.json path.
func SaveClientIndex(manifestPath string, m *Manifest) error {
	idx := BuildClientIndex(m)
	data, err := json.Marshal(idx)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(filepath.Join(filepath.Dir(manifestPath), clientIndexFileName), data, 0o644)
}

/*
Windows가 파일 이름으로 쓸 수 없는 이름들.

`CON`·`PRN`·`AUX`·`NUL`·`COM1`~`COM9`·`LPT1`~`LPT9`는 DOS 시절의 장치 이름이라
Windows에서 파일 이름이 될 수 없다. 그런데 이것들이 **실제 미국 티커**다 —
CON(Concentra)·PRN이 그렇고, 그 로고 파일이 저장소에 들어가면 Windows에서
체크아웃이 통째로 실패한다(실측: `git add`가 `open("…/CON.png"): No such file
or directory`로 멈췄다).

그래서 이 이름들은 **목록에 적지 않는다.** 화면은 그 두 종목만 예전처럼 원격
URL로 떨어진다 — 11,205개 중 둘이라, 저장소를 Windows에서 못 여는 값을 치를
이유가 없다.

붙임 문자를 바꿔 저장하는 방법(`CON_.png`)도 있지만, 그러면 「경로는 심볼로
되짚는다」는 이 파일의 전제가 깨져 예외를 화면 쪽에도 심어야 한다.
*/
var windowsReservedNames = map[string]bool{
	"CON": true, "PRN": true, "AUX": true, "NUL": true,
	"COM1": true, "COM2": true, "COM3": true, "COM4": true, "COM5": true,
	"COM6": true, "COM7": true, "COM8": true, "COM9": true,
	"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true, "LPT5": true,
	"LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
}

// IsWindowsReservedName reports whether a symbol cannot be a file name on Windows.
func IsWindowsReservedName(symbol string) bool {
	return windowsReservedNames[strings.ToUpper(strings.TrimSpace(symbol))]
}
