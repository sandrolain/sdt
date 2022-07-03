package utils

import (
	"bytes"
	"image"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
)

func ReadQRCodeImage(data []byte) (string, error) {
	reader := bytes.NewReader(data)
	img, _, err := image.Decode(reader)
	if err != nil {
		return "", err
	}

	// prepare BinaryBitmap
	bmp, _ := gozxing.NewBinaryBitmapFromImage(img)

	// decode image
	qrReader := qrcode.NewQRCodeReader()
	result, _ := qrReader.Decode(bmp, nil)

	return result.String(), nil
}
