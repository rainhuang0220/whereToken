package main

import (
	"bytes"
	"image"
	"image/jpeg"
	"math"
	"sort"
)

const (
	heightWidth    = 768
	maxHeightBytes = 180 * 1024
)

// encodeHeightJPEG writes a contrast-stretched displacement field. The
// browser shader derives normals and roughness from this one map.
func encodeHeightJPEG(disp image.Image) ([]byte, image.Point, error) {
	b := disp.Bounds()
	sw, sh := b.Dx(), b.Dy()
	dw := heightWidth
	dh := int(math.Round(float64(sh) * float64(dw) / float64(sw)))
	if dh < 2 {
		dh = 2
	}
	field := stretchHeight(resizeWrap(heightPlane(disp), sw, sh, dw, dh))
	img := image.NewGray(image.Rect(0, 0, dw, dh))
	for i, v := range field {
		img.Pix[i] = uint8(math.Round(v * 255))
	}
	var payload []byte
	var err error
	for quality := 78; quality >= 58; quality -= 4 {
		buf := bytes.Buffer{}
		if err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
			return nil, image.Point{}, err
		}
		payload = buf.Bytes()
		if len(payload) <= maxHeightBytes {
			break
		}
	}
	return payload, image.Pt(dw, dh), nil
}

func stretchHeight(src []float64) []float64 {
	if len(src) == 0 {
		return src
	}
	sample := make([]float64, 0, len(src)/16+1)
	for i := 0; i < len(src); i += 16 {
		sample = append(sample, src[i])
	}
	sort.Float64s(sample)
	lo := sample[int(0.03*float64(len(sample)))]
	hi := sample[int(0.97*float64(len(sample)-1))]
	span := hi - lo
	if span < 1e-4 {
		span = 1
	}
	out := make([]float64, len(src))
	for i, v := range src {
		t := (v - lo) / span
		if t < 0 {
			t = 0
		}
		if t > 1 {
			t = 1
		}
		out[i] = t
	}
	return out
}
