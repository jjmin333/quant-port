package stocklogo

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// 100×100 로고를 만든다: 배경 bg, 왼쪽 위 n×n칸만 fg.
func writeLogo(t *testing.T, bg, fg color.NRGBA, n int) string {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			c := bg
			if x < n && y < n {
				c = fg
			}
			img.SetNRGBA(x, y, c)
		}
	}
	p := filepath.Join(t.TempDir(), "logo.png")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLogoTone(t *testing.T) {
	white := color.NRGBA{255, 255, 255, 255}
	clear := color.NRGBA{0, 0, 0, 0}
	gray := color.NRGBA{90, 90, 90, 255}
	yellow := color.NRGBA{255, 220, 0, 255}

	cases := []struct {
		name string
		bg   color.NRGBA
		fg   color.NRGBA
		n    int // fg 면적 = n²/100 %
		want string
	}{
		{"흰 판 위 짙은 로고", white, gray, 30, ""},
		{"흰 판 위 가는 회색 글자 (LITE·CLS)", white, gray, 10, ""},
		{"흰 판 위 노란 로고 (CAVA)", white, yellow, 30, ""},
		{"투명 배경 위 흰 그림 (QQQ)", clear, white, 50, ToneLight},
		{"투명 글자가 뚫린 흰 도형 (ADI·RBLX)", white, clear, 30, ToneLight},
		{"순백 사각형 (KNX)", white, white, 0, ToneBlank},
	}
	for _, c := range cases {
		if got := LogoTone(writeLogo(t, c.bg, c.fg, c.n)); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
