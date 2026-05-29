package core

import (
	"image"
	"image/jpeg"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"os"

	"golang.org/x/image/draw"
)

func ResizeJPEG(scrPath, dstPath string, width, quality int) error {
	file, err := os.Open(scrPath)
	if err != nil {
		return err
	}
	defer file.Close()

	srcImg, _, err := image.Decode(file)
	if err != nil {
		return err
	}

	bounds := srcImg.Bounds()
	originalWidth := bounds.Dx()
	originalHeight := bounds.Dy()

	newHeight := (originalHeight * width) / originalWidth

	dstImg := image.NewRGBA(image.Rect(0, 0, width, newHeight))

	draw.BiLinear.Scale(dstImg, dstImg.Bounds(), srcImg, srcImg.Bounds(), draw.Over, nil)

	casheFile, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer casheFile.Close()

	options := &jpeg.Options{
		Quality: quality,
	}

	err = jpeg.Encode(casheFile, dstImg, options)
	if err != nil {
		log.Printf("Encoding error %v\n", err)
		return err
	}

	return nil
}
