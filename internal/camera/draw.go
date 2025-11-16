package camera

import (
	"image"
	"image/color"
	"image/draw"
)

// drawBoxes draws rectangle outlines over an RGBA image.
func drawBoxes(dst *image.RGBA, rects []image.Rectangle, col color.RGBA) {
	for _, r := range rects {
		for x := r.Min.X; x < r.Max.X; x++ {
			if inside(dst, x, r.Min.Y) {
				dst.SetRGBA(x, r.Min.Y, col)
			}
			if inside(dst, x, r.Max.Y-1) {
				dst.SetRGBA(x, r.Max.Y-1, col)
			}
		}
		for y := r.Min.Y; y < r.Max.Y; y++ {
			if inside(dst, r.Min.X, y) {
				dst.SetRGBA(r.Min.X, y, col)
			}
			if inside(dst, r.Max.X-1, y) {
				dst.SetRGBA(r.Max.X-1, y, col)
			}
		}
	}
}

func inside(img *image.RGBA, x, y int) bool {
	b := img.Bounds()
	return x >= b.Min.X && x < b.Max.X && y >= b.Min.Y && y < b.Max.Y
}

func toRGBA(src image.Image) *image.RGBA {
	b := src.Bounds()
	dst := image.NewRGBA(b)
	draw.Draw(dst, b, src, b.Min, draw.Src)
	return dst
}
