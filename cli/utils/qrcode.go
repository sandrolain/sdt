package utils

import (
	"bytes"
	"image"
	_ "image/jpeg"
	"image/png"

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

func GenerateQRCodeImage(data string, size int) ([]byte, error) {
	enc := qrcode.NewQRCodeWriter()
	img, _ := enc.EncodeWithoutHint(data, gozxing.BarcodeFormat_QR_CODE, size, size)
	out := new(bytes.Buffer)
	err := png.Encode(out, img)
	if err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
