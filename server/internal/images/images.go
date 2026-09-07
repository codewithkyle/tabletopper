// Package images is the encoder every stored image goes through, and the crop
// that makes a thumbnail out of one.
//
// It exists because two callers now need the same encoder: the upload handlers,
// which write avatars and journal pictures inside a request, and the map tiler,
// which writes hundreds of tiles and a preview outside of one. A second copy of
// five lines would be a second quality setting to forget about.
package images

import (
	"bytes"
	"image"

	"github.com/chai2010/webp"
	"github.com/disintegration/imaging"
)

const (
	// quality is the WebP setting every stored image is encoded at.
	quality = 75

	// MapPreviewSize is the square a map's card shows. It is a constant here
	// rather than at either caller because the preview is written by the
	// tiler and rendered by the assets page, and a disagreement between the
	// two is a thumbnail that is resampled by the browser.
	MapPreviewSize = 256
)

// Square crops and scales img to a size-by-size square, centred.
func Square(img image.Image, size int) image.Image {
	return imaging.Fill(img, size, size, imaging.Center, imaging.Lanczos)
}

// Fit scales img down to sit inside a size-by-size box WITHOUT CROPPING IT, and
// leaves anything already smaller alone.
//
// IT IS THE ONE A TOKEN NEEDS, and the difference from Square is the whole
// reason it exists. Square centre-crops, which is right for a portrait -- a face
// is in the middle and a thumbnail is square -- and wrong for a longboat, which
// it would turn into a square of hull. A token is placed on a map at the shape
// it was drawn at, so the shape is the thing being preserved.
//
// It never enlarges: imaging.Fit scales down only, so a 64-pixel token stays 64
// pixels rather than being blown up to the box and stored blurry.
func Fit(img image.Image, size int) image.Image {
	return imaging.Fit(img, size, size, imaging.Lanczos)
}

// EncodeWebP encodes img as lossy WebP.
//
// AN *image.RGBA GOES STRAIGHT TO THE ENCODER and every other type is copied
// into one first. It costs a full-size allocation and a pass over the pixels,
// which is worth knowing when the caller is producing images in bulk and can
// choose the type it produces them in.
func EncodeWebP(img image.Image) ([]byte, error) {
	var out bytes.Buffer
	if err := webp.Encode(&out, img, &webp.Options{Quality: quality}); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
