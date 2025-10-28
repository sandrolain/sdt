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
	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		return "", err
	}

	// decode image
	qrReader := qrcode.NewQRCodeReader()
	result, err := qrReader.Decode(bmp, nil)
	if err != nil {
		return "", err
	}

	return result.String(), nil
}

func GenerateQRCodeImage(data string, size int) ([]byte, error) {
	enc := qrcode.NewQRCodeWriter()
	img, err := enc.EncodeWithoutHint(data, gozxing.BarcodeFormat_QR_CODE, size, size)
	if err != nil {
		return nil, err
	}
	out := new(bytes.Buffer)
	err = png.Encode(out, img)
	if err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
