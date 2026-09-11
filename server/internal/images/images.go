






package images

import (
	"bytes"
	"image"

	"github.com/chai2010/webp"
	"github.com/disintegration/imaging"
)

const (
	
	
	
	
	DefaultQuality = 75

	
	
	
	
	MapPreviewSize = 256
)


func Square(img image.Image, size int) image.Image {
	return imaging.Fill(img, size, size, imaging.Center, imaging.Lanczos)
}












func Fit(img image.Image, size int) image.Image {
	return imaging.Fit(img, size, size, imaging.Lanczos)
}


func EncodeWebP(img image.Image) ([]byte, error) {
	return EncodeWebPAt(img, DefaultQuality)
}














func EncodeWebPAt(img image.Image, quality int) ([]byte, error) {
	var out bytes.Buffer
	if err := webp.Encode(&out, img, &webp.Options{Quality: float32(quality)}); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
