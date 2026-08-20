package imageutil

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
)

// CropRelative extracts a sub-rectangle using normalized coordinates (0–1).
func CropRelative(srcPath, dstPath string, x, y, w, h float64) error {
	f, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return err
	}
	sub, err := subImage(img, x, y, w, h)
	if err != nil {
		return err
	}
	return encodePNG(dstPath, sub)
}

func subImage(img image.Image, x, y, w, h float64) (image.Image, error) {
	b := img.Bounds()
	x0 := b.Min.X + int(x*float64(b.Dx()))
	y0 := b.Min.Y + int(y*float64(b.Dy()))
	x1 := b.Min.X + int((x+w)*float64(b.Dx()))
	y1 := b.Min.Y + int((y+h)*float64(b.Dy()))
	if x0 < b.Min.X {
		x0 = b.Min.X
	}
	if y0 < b.Min.Y {
		y0 = b.Min.Y
	}
	if x1 > b.Max.X {
		x1 = b.Max.X
	}
	if y1 > b.Max.Y {
		y1 = b.Max.Y
	}
	if x1 <= x0 || y1 <= y0 {
		return nil, fmt.Errorf("invalid crop region")
	}
	rect := image.Rect(x0, y0, x1, y1)
	if si, ok := img.(interface {
		SubImage(r image.Rectangle) image.Image
	}); ok {
		return si.SubImage(rect), nil
	}
	out := image.NewRGBA(image.Rect(0, 0, x1-x0, y1-y0))
	for py := y0; py < y1; py++ {
		for px := x0; px < x1; px++ {
			out.Set(px-x0, py-y0, img.At(px, py))
		}
	}
	return out, nil
}

func encodePNG(dstPath string, img image.Image) error {
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer out.Close()
	return png.Encode(out, img)
}
