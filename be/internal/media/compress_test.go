package media

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math/rand"
	"testing"
)

// noisy makes an image that compresses badly — the worst case for the byte budget.
func noisy(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	rng := rand.New(rand.NewSource(1))
	for i := range img.Pix {
		img.Pix[i] = byte(rng.Intn(256))
	}
	return img
}

func TestFitSize(t *testing.T) {
	cases := []struct{ w, h, max, ew, eh int }{
		{4000, 3000, 1600, 1600, 1200},
		{3000, 4000, 1600, 1200, 1600},
		{800, 600, 1600, 800, 600}, // 이미 작으면 그대로 — 키우지 않는다
		{10000, 10, 1600, 1600, 2},
	}
	for _, c := range cases {
		w, h := FitSize(c.w, c.h, c.max)
		if w != c.ew || h != c.eh {
			t.Errorf("FitSize(%d,%d,%d)=%d,%d want %d,%d", c.w, c.h, c.max, w, h, c.ew, c.eh)
		}
	}
}

func TestCompressBigNoisyPNGStaysUnderHardCap(t *testing.T) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, noisy(3000, 2000)); err != nil {
		t.Fatal(err)
	}
	out, err := CompressImage(buf.Bytes())
	if err != nil {
		t.Fatalf("compress: %v", err)
	}
	if len(out.Data) > CompressHardCap {
		t.Fatalf("저장본이 1MB를 넘었다: %d", len(out.Data))
	}
	if out.ContentType != "image/jpeg" {
		t.Fatalf("type %s", out.ContentType)
	}
	if out.Width > CompressMaxEdge || out.Height > CompressMaxEdge {
		t.Fatalf("긴 변이 %d를 넘었다: %dx%d", CompressMaxEdge, out.Width, out.Height)
	}
}

func TestCompressTypicalPhotoHitsTarget(t *testing.T) {
	// 부드러운 그라데이션 — 실제 사진·캡처에 가까운 경우는 목표(200KB) 안에 든다.
	img := image.NewRGBA(image.Rect(0, 0, 4000, 3000))
	for y := 0; y < 3000; y++ {
		for x := 0; x < 4000; x++ {
			img.Set(x, y, color.RGBA{uint8(x / 16), uint8(y / 12), 120, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	out, err := CompressImage(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Data) > CompressTargetBytes {
		t.Fatalf("보통 사진은 목표 안에 들어야 한다: %d > %d", len(out.Data), CompressTargetBytes)
	}
	if out.Width != 1600 || out.Height != 1200 {
		t.Fatalf("size %dx%d", out.Width, out.Height)
	}
}

func TestCompressRejectsNonImage(t *testing.T) {
	if _, err := CompressImage([]byte("hello, not an image at all")); err == nil {
		t.Fatal("이미지가 아니면 거부해야 한다")
	}
}

func TestApplyOrientationRotates90(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 3, 2)) // 가로로 긴 원본
	src.Set(0, 0, color.RGBA{255, 0, 0, 255})    // 왼쪽 위 빨강
	out := applyOrientation(src, 6)              // 시계 방향 90°
	b := out.Bounds()
	if b.Dx() != 2 || b.Dy() != 3 {
		t.Fatalf("90° 회전이면 가로·세로가 바뀐다: %dx%d", b.Dx(), b.Dy())
	}
	if r, _, _, _ := out.At(1, 0).RGBA(); r>>8 != 255 {
		t.Fatalf("왼쪽 위가 오른쪽 위로 가야 한다")
	}
}

func TestJPEGOrientationParsesExif(t *testing.T) {
	// SOI + APP1(Exif, 빅엔디언 TIFF, IFD0에 Orientation=6 하나) + EOI
	tiff := []byte{'M', 'M', 0, 42, 0, 0, 0, 8, 0, 1, 0x01, 0x12, 0, 3, 0, 0, 0, 1, 0, 6, 0, 0, 0, 0, 0, 0, 0, 0}
	seg := append([]byte("Exif\x00\x00"), tiff...)
	size := len(seg) + 2
	data := []byte{0xFF, 0xD8, 0xFF, 0xE1, byte(size >> 8), byte(size)}
	data = append(data, seg...)
	data = append(data, 0xFF, 0xD9)
	if got := jpegOrientation(data); got != 6 {
		t.Fatalf("orientation %d, want 6", got)
	}
	if got := jpegOrientation([]byte{0xFF, 0xD8, 0xFF, 0xD9}); got != 1 {
		t.Fatalf("EXIF 없으면 1: %d", got)
	}
}
