package jpegfast

import (
	"bytes"
	"errors"
	"image"
	"image/jpeg"
)

type Subsampling int

const (
	Subsample444 Subsampling = iota
	Subsample422
	Subsample420
	Subsample440
	Subsample411
	SubsampleGray
)

type Flags uint32

const (
	FlagAccurateDCT Flags = 1 << iota
	FlagFastDCT
	FlagFastUpsample
	FlagNoRealloc
)

type EncodeConfig struct {
	Quality     int
	Subsampling Subsampling
	Flags       Flags
}

var ErrInvalidInput = errors.New("invalid dimensions or buffer length")

func EncodeBGR(bgr []byte, width, height int, cfg EncodeConfig) ([]byte, error) {
	if width <= 0 || height <= 0 || len(bgr) != width*height*3 {
		return nil, ErrInvalidInput
	}
	if cfg.Quality < 1 {
		cfg.Quality = 1
	}
	if cfg.Quality > 100 {
		cfg.Quality = 100
	}
	img := bgrToNRGBA(bgr, width, height)
	return encodeNRGBA(img, cfg.Quality)
}

func bgrToNRGBA(bgr []byte, width, height int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	dst := img.Pix
	stride := img.Stride
	for y := 0; y < height; y++ {
		rowDst := dst[y*stride : y*stride+width*4]
		rowSrc := bgr[y*width*3 : y*width*3+width*3]
		for x := 0; x < width; x++ {
			b := rowSrc[x*3+0]
			g := rowSrc[x*3+1]
			r := rowSrc[x*3+2]
			rowDst[x*4+0] = r
			rowDst[x*4+1] = g
			rowDst[x*4+2] = b
			rowDst[x*4+3] = 0xFF
		}
	}
	return img
}

func encodeNRGBA(img image.Image, quality int) ([]byte, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
