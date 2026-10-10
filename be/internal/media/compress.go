package media

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	stddraw "image/draw"
	"image/jpeg"

	// Decoders registered for image.Decode.
	_ "image/gif"
	_ "image/png"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

/*
커뮤니티 이미지는 **저장 전에 다시 굽는다** (2026-09-27 사용자 결정).

「서버에 저장되는 이미지는 아무리 커도 1MB, 평균 100~200KB」. 휴대폰 사진은 3~8MB, 화면
캡처 PNG는 1~3MB라 그대로 받으면 디스크(무료 인스턴스)와 목록 화면 트래픽이 먼저 터진다.

  1. 긴 변을 CompressMaxEdge(1600px)로 줄인다 — 글 본문 폭(최대 ~760px)의 2배, 레티나에서도 충분.
  2. JPEG로 품질 사다리(82→74→66→58)를 내려가며 CompressTargetBytes(200KB) 안에 드는 첫 값.
  3. 그래도 크면 긴 변 1024px로 한 번 더. 끝까지 1MB(CompressHardCap)를 넘으면 거부.

다시 굽는 부수 효과가 둘 있고 둘 다 의도다:
  - **EXIF가 지워진다** — 휴대폰 사진의 GPS 좌표가 공개 글에 실려 나가지 않는다.
    대신 회전 정보(Orientation)도 지워지므로 **굽기 전에 픽셀을 돌려 둔다**(안 하면 세로 사진이 눕는다).
  - 움직이는 GIF는 첫 장만 남는다(정지 이미지).
투명 배경(PNG)은 흰색 위에 얹는다 — JPEG에는 알파가 없다.
*/

const (
	CompressMaxEdge      = 1600
	CompressFallbackEdge = 1024
	CompressTargetBytes  = 200 << 10
	CompressHardCap      = 1 << 20
)

var compressQualities = []int{82, 74, 66, 58}

// Compressed is a re-encoded image ready for Store.Put.
type Compressed struct {
	Data        []byte
	ContentType string
	Width       int
	Height      int
}

// CompressImage decodes, orients, downsizes and re-encodes data as JPEG.
func CompressImage(data []byte) (Compressed, error) {
	if _, err := SniffImageType(data); err != nil {
		return Compressed{}, err
	}
	src, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return Compressed{}, fmt.Errorf("%w: decode: %v", ErrUnsupportedType, err)
	}
	if format == "jpeg" {
		src = applyOrientation(src, jpegOrientation(data))
	}

	for _, edge := range []int{CompressMaxEdge, CompressFallbackEdge} {
		img := flattenAndFit(src, edge)
		var best []byte
		for _, q := range compressQualities {
			var buf bytes.Buffer
			if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: q}); err != nil {
				return Compressed{}, err
			}
			best = buf.Bytes()
			if len(best) <= CompressTargetBytes {
				break
			}
		}
		if len(best) <= CompressTargetBytes || edge == CompressFallbackEdge {
			if len(best) > CompressHardCap {
				return Compressed{}, ErrTooLarge
			}
			b := img.Bounds()
			return Compressed{Data: best, ContentType: "image/jpeg", Width: b.Dx(), Height: b.Dy()}, nil
		}
	}
	return Compressed{}, ErrTooLarge // unreachable: the fallback edge always returns
}

// FitSize scales (w, h) so the long edge is at most maxEdge, keeping the ratio.
func FitSize(w, h, maxEdge int) (int, int) {
	long := w
	if h > long {
		long = h
	}
	if long <= maxEdge || long == 0 {
		return w, h
	}
	nw := int(float64(w)*float64(maxEdge)/float64(long) + 0.5)
	nh := int(float64(h)*float64(maxEdge)/float64(long) + 0.5)
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}
	return nw, nh
}

// flattenAndFit draws src onto an opaque white RGBA canvas at the fitted size.
func flattenAndFit(src image.Image, maxEdge int) *image.RGBA {
	b := src.Bounds()
	w, h := FitSize(b.Dx(), b.Dy(), maxEdge)
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	stddraw.Draw(dst, dst.Bounds(), &image.Uniform{C: color.White}, image.Point{}, stddraw.Src)
	if w == b.Dx() && h == b.Dy() {
		stddraw.Draw(dst, dst.Bounds(), src, b.Min, stddraw.Over)
		return dst
	}
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)
	return dst
}

// jpegOrientation reads the EXIF Orientation tag (1..8) from a JPEG, or 1.
// 필요한 만큼만 읽는 최소 파서다 — APP1 Exif → TIFF 헤더 → IFD0에서 태그 0x0112.
func jpegOrientation(data []byte) int {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return 1
	}
	i := 2
	for i+4 <= len(data) {
		if data[i] != 0xFF {
			return 1
		}
		marker := data[i+1]
		if marker == 0xDA || marker == 0xD9 { // start of scan / end — EXIF would have come before
			return 1
		}
		size := int(binary.BigEndian.Uint16(data[i+2 : i+4]))
		if size < 2 || i+2+size > len(data) {
			return 1
		}
		seg := data[i+4 : i+2+size]
		if marker == 0xE1 && len(seg) >= 14 && string(seg[:6]) == "Exif\x00\x00" {
			return tiffOrientation(seg[6:])
		}
		i += 2 + size
	}
	return 1
}

func tiffOrientation(t []byte) int {
	if len(t) < 8 {
		return 1
	}
	var bo binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return 1
	}
	ifd := int(bo.Uint32(t[4:8]))
	if ifd+2 > len(t) {
		return 1
	}
	n := int(bo.Uint16(t[ifd : ifd+2]))
	for k := 0; k < n; k++ {
		e := ifd + 2 + k*12
		if e+12 > len(t) {
			return 1
		}
		if bo.Uint16(t[e:e+2]) == 0x0112 {
			v := int(bo.Uint16(t[e+8 : e+10]))
			if v >= 1 && v <= 8 {
				return v
			}
			return 1
		}
	}
	return 1
}

// applyOrientation returns src transformed so it displays upright (EXIF 1..8).
func applyOrientation(src image.Image, o int) image.Image {
	if o <= 1 || o > 8 {
		return src
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	swap := o >= 5
	dw, dh := w, h
	if swap {
		dw, dh = h, w
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var dx, dy int
			switch o {
			case 2: // mirror horizontal
				dx, dy = w-1-x, y
			case 3: // rotate 180
				dx, dy = w-1-x, h-1-y
			case 4: // mirror vertical
				dx, dy = x, h-1-y
			case 5: // transpose
				dx, dy = y, x
			case 6: // rotate 90 CW
				dx, dy = h-1-y, x
			case 7: // transverse
				dx, dy = h-1-y, w-1-x
			case 8: // rotate 270 CW
				dx, dy = y, w-1-x
			}
			dst.Set(dx, dy, src.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}
