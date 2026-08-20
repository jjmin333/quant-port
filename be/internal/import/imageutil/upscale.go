package imageutil

import (
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
)

// UpscaleFile scales an image by an integer factor (nearest-neighbor).
func UpscaleFile(srcPath, dstPath string, factor int) error {
	if factor < 2 {
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
	up := nearestUpscale(img, factor)
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
		return png.Encode(out, up)
	default:
		return jpeg.Encode(out, up, &jpeg.Options{Quality: 95})
	}
}

func nearestUpscale(src image.Image, factor int) image.Image {
	b := src.Bounds()
	w, h := b.Dx()*factor, b.Dy()*factor
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(x, y, src.At(b.Min.X+x/factor, b.Min.Y+y/factor))
		}
	}
	return dst
}
