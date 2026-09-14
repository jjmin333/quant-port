package stocklogo

import (
	"image"
	_ "image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
)

/*
로고의 밝기 판별 — **흰 로고를 흰 배경에 얹지 않기 위해서**.

해외 로고를 내려받고 나서 몇 종목이 카드에서 통째로 사라졌다. 실측(08-31, 캔버스로
픽셀을 세어 확인):

	SOXL   투명 95% · 불투명 픽셀 평균 밝기 235 · 그중 83%가 흰색에 가까움
	QQQ    투명 69% · 평균 255 · 100% 흰색
	SOXS   투명  0% · 평균 250 ·  96% 흰색
	NVDA   투명 60% · 평균 159 ·   0% 흰색   ← 정상적으로 보이던 쪽

즉 **없는 그림이 아니라 흰 그림**이었다. 밝은 테마의 흰 카드 위에서는 있어도 안 보인다.
화면에서 이걸 알아내려면 그릴 때마다 픽셀을 읽어야 하는데, 목록 한 줄마다 캔버스를
돌릴 수는 없다. 그래서 **받을 때 한 번 재서 매니페스트에 적어 둔다** — 화면은 그
표시만 보고 어두운 판을 깔지 결정한다.

밝기는 sRGB 상대휘도 근사(0.2126R + 0.7152G + 0.0722B)를 쓴다. 정확한 색 공간 변환이
필요한 판단이 아니다 — "흰색에 가까운가"만 가르면 된다.
*/

/*
판정 기준: **흰 배경에 얹었을 때 실제로 보이는 면적이 얼마나 되는가.**

처음에는 "불투명 픽셀 중 흰색에 가까운 비율"로 쟀는데, 15,504개 중 6,459개(42%)가
걸렸다. 국내 로고가 전부 딸려 왔기 때문이다 — 네이버 로고는 **흰 판 위에 작은 색
마크**라 흰 픽셀이 원래 많다. 그 로고들은 지금도 잘 보인다. 즉 흰 픽셀이 많다는 것과
안 보인다는 것은 다른 이야기였다.

그래서 질문을 바꾼다. 알파를 흰색 위에 합성한 뒤 **눈에 띄게 어두운 픽셀이 전체 면적의
몇 %인가**를 센다. 그 값이 거의 0이면 흰 카드 위에서 아무것도 안 보인다는 뜻이고,
그게 정확히 고치려는 증상이다.

실측(08-31, 브라우저 캔버스로 확인한 값과 같은 계열):

	SOXL  투명 95% · 남은 5%의 83%가 흰색  →  보이는 면적 ≈ 0.9%   ← 안 보임
	QQQ   투명 69% · 남은 것의 100%가 흰색  →  ≈ 0%                 ← 안 보임
	SOXS  투명  0% · 96%가 흰색             →  ≈ 4%                 ← 거의 안 보임
	NVDA  투명 60% · 흰색 0%                →  ≈ 40%                ← 정상
	네이버 흰 판 로고                        →  색 마크 면적만큼      ← 정상
*/
/*
안 보이는 로고에도 **두 종류**가 있고, 처방이 다르다.

	SOXL  투명 94.6% · 보이는 면적 0.8%   투명 배경 위의 흰 그림
	QQQ   투명 68.8% · 0%                  〃
	SOXS  투명  0%   · 3.6%                흰 사각형 이미지 그 자체

앞의 둘은 어두운 판을 깔면 흰 그림이 드러난다. 뒤는 **이미지가 판을 통째로 덮는다** —
불투명한 흰 사각형이라 뒤에 무엇을 깔든 보이지 않고, 화면에는 어두운 동그라미 안의
흰 상자만 남는다. 실제로 그렇게 보였다.

966개를 갈라 보니 투명 배경 쪽이 487개, 불투명 흰 이미지가 364개, 그 사이가 115개다.
가운데 구간도 60~90%가 불투명한 옅은 색이라 판 위에서 옅은 덩어리로 보이므로, **판이
확실히 통하는 쪽(투명 40% 이상)만** 판을 깔고 나머지는 「쓸 수 있는 로고가 없다」로
본다 — 그쪽은 모노그램이 훨씬 읽기 쉽다.
*/
const (
	// 흰 배경 위에서 이보다 어두우면 "보인다"고 센다.
	logoVisibleLuminance = 200
	/*
		보이는 면적이 이 비율에 못 미치면 흰 배경에서 사실상 안 보인다.

		5%는 문제가 된 쪽(0~4%)과 멀쩡한 쪽(40% 이상, 국내 판 로고도 두 자릿수) 사이의
		아주 넓은 골짜기다. 경계에서 흔들릴 표본이 없다.
	*/
	logoMinVisibleRatio = 0.05
	/*
		원형 네이버 로고(160×160, 모서리 20% 투명)는 글자 면적이 5%에 못 미친다.

		도우인시스(484120) 실측: 보이는 면적 4.50% · 투명 20.61% — 읽히는데 blank로
		떨어져 모노그램만 나왔다. SOXS(투명 0%)와 달리 배경이 비어 있으므로, 투명
		15% 이상이면서 어두운 픽셀이 3% 이상이면 정상 로고로 본다.
	*/
	logoMinVisibleSmallMark   = 0.03
	logoMinTransparentSmallMark = 0.15
	/*
		어두운 판이 통하려면 배경이 이만큼은 비어 있어야 한다.

		40%는 실측 분포의 골짜기다 — 판이 통하는 쪽은 68~95%이고, 안 통하는 쪽은 0%에
		몰려 있다.
	*/
	logoMinTransparentRatio = 0.40
	// 알파가 이보다 낮으면 배경으로 본다.
	logoOpaqueAlpha = 16
)

// Logo tone classifications. 빈 문자열은 "정상" — 화면이 지금까지와 같게 그린다.
const (
	// ToneLight: 투명 배경 위의 흰 그림. 어두운 판을 깔면 드러난다.
	ToneLight = "light"
	// ToneBlank: 흰 배경에서 안 보이는데 판도 통하지 않는다. 로고가 없는 것으로 친다.
	ToneBlank = "blank"
)

// LogoTone classifies a logo by what it needs to be legible on a white card.
//
// 빈 문자열은 "판단하지 않음"이다 — 읽기 실패와 정상 로고가 함께 들어간다.
func LogoTone(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	img, err := decodeLogo(f)
	if err != nil {
		return ""
	}

	bounds := img.Bounds()
	total := bounds.Dx() * bounds.Dy()
	if total <= 0 {
		return ""
	}

	visible, transparent := 0, 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			// RGBA()는 16비트 **프리멀티플라이드** 값을 준다 — 이미 알파가 곱해져 있다.
			r16, g16, b16, a16 := img.At(x, y).RGBA()
			if a16>>8 < logoOpaqueAlpha {
				transparent++
				continue // 완전 투명 — 흰 배경이 그대로 보인다.
			}
			/*
				흰색 위에 합성: out = src + white*(1-alpha).
				프리멀티플라이드 값이라 src를 그대로 더하면 된다.
			*/
			inv := 0xffff - a16
			r := float64(r16+inv) / 0xffff * 255
			g := float64(g16+inv) / 0xffff * 255
			b := float64(b16+inv) / 0xffff * 255
			if 0.2126*r+0.7152*g+0.0722*b < logoVisibleLuminance {
				visible++
			}
		}
	}

	if float64(visible)/float64(total) >= logoMinVisibleRatio {
		return "" // 흰 배경에서도 충분히 보인다 — 손댈 이유가 없다.
	}
	if float64(transparent)/float64(total) >= logoMinTransparentRatio {
		return ToneLight
	}
	if float64(transparent)/float64(total) >= logoMinTransparentSmallMark &&
		float64(visible)/float64(total) >= logoMinVisibleSmallMark {
		return "" // 원형 네이버 로고 — 글자 면적은 작지만 실제로 읽힌다.
	}
	return ToneBlank
}

// decodeLogo reads PNG first (everything we store is PNG) and falls back to the
// generic decoder for anything a provider mislabelled.
func decodeLogo(f *os.File) (image.Image, error) {
	img, err := png.Decode(f)
	if err == nil {
		return img, nil
	}
	if _, seekErr := f.Seek(0, 0); seekErr != nil {
		return nil, err
	}
	img, _, err = image.Decode(f)
	return img, err
}

/*
RetoneManifest는 **이미 받아 둔 파일만 다시 재서** 매니페스트의 톤 표시를 채운다.

받는 자리에서 재는 것이 원칙이지만(`processUS`·`processDomestic`), 이 판별을 붙이기
전에 내려받은 15,000여 개가 이미 있다. 그것들 때문에 전부 다시 받으면 100MB를 다시
끌어오고 FMP·네이버에 15,000번을 더 묻는 셈이라, 로컬 파일만 훑는 길을 따로 둔다.

경로가 없는 항목(원격 URL로 떨어지는 종목)과 파일이 사라진 항목은 건드리지 않는다.
*/
func RetoneManifest(m *Manifest, outDir string) (checked, light int) {
	if m == nil {
		return 0, 0
	}
	for sym, entry := range m.Entries {
		if entry.Path == "" {
			continue
		}
		// 매니페스트 경로는 `/logos/kr/005930.png` 꼴의 웹 경로다. 파일 위치로 바꾼다.
		rel := strings.TrimPrefix(entry.Path, "/logos/")
		if rel == entry.Path {
			continue
		}
		path := filepath.Join(outDir, filepath.FromSlash(rel))
		if _, err := os.Stat(path); err != nil {
			continue
		}
		checked++
		tone := LogoTone(path)
		if tone == "light" {
			light++
		}
		if entry.Tone != tone {
			entry.Tone = tone
			m.Entries[sym] = entry
		}
	}
	return checked, light
}
