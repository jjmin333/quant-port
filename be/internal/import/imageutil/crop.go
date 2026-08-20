package imageutil

import (
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/enact/quant-port/be/internal/model"
)

func CropImageFile(srcPath, dstPath string, crop *model.ImportCropRect) error {
	if crop == nil || crop.W <= 0 || crop.H <= 0 {
		return copyFile(srcPath, dstPath)
	}

	f, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return err
	}
	b := img.Bounds()
	x0 := int(crop.X * float64(b.Dx()))
	y0 := int(crop.Y * float64(b.Dy()))
	x1 := int((crop.X + crop.W) * float64(b.Dx()))
	y1 := int((crop.Y + crop.H) * float64(b.Dy()))
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x1 > b.Dx() {
		x1 = b.Dx()
	}
	if y1 > b.Dy() {
		y1 = b.Dy()
	}
	if x1 <= x0 || y1 <= y0 {
		return copyFile(srcPath, dstPath)
	}

	rect := image.Rect(b.Min.X+x0, b.Min.Y+y0, b.Min.X+x1, b.Min.Y+y1)
	var sub image.Image
	if si, ok := img.(interface {
		SubImage(r image.Rectangle) image.Image
	}); ok {
		sub = si.SubImage(rect)
	} else {
		sub = image.NewRGBA(image.Rect(0, 0, x1-x0, y1-y0))
		dst := sub.(*image.RGBA)
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				dst.Set(x-x0, y-y0, img.At(x, y))
			}
		}
	}

	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer out.Close()

	ext := strings.ToLower(filepath.Ext(dstPath))
	switch ext {
	case ".png":
		return png.Encode(out, sub)
	default:
		return jpeg.Encode(out, sub, &jpeg.Options{Quality: 92})
	}
}

func copyFile(src, dst string) error {
	in, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(dst, in, 0o644); err != nil {
		return err
	}
	return nil
}

func PreprocessForOCR(srcPath, dstPath string) error {
	f, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	b := img.Bounds()
	gray := image.NewGray(b)
	var lumSum int64
	var count int64
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			lum := (299*r + 587*g + 114*bl) / 1000
			y8 := uint8(lum >> 8)
			gray.SetGray(x, y, color.Gray{Y: y8})
			lumSum += int64(y8)
			count++
		}
	}
	// Dark-theme screenshots (e.g. securities apps) read better inverted for Tesseract.
	if count > 0 && lumSum/int64(count) < 96 {
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				g := gray.GrayAt(x, y).Y
				gray.SetGray(x, y, color.Gray{Y: 255 - g})
			}
		}
	}
	// Binarize lightly so dim gray qty text (e.g. "4주") separates from background.
	const thresh = uint8(140)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			g := gray.GrayAt(x, y).Y
			if g < thresh {
				g = 0
			} else {
				g = 255
			}
			gray.SetGray(x, y, color.Gray{Y: g})
		}
	}

	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer out.Close()
	return png.Encode(out, gray)
}
