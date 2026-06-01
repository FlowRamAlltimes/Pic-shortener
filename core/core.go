package core

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"net/http"
	"os"

	"golang.org/x/image/draw"
)

type MediaError struct {
	Op      string
	Code    int
	Message string
	Err     error
}

func (e *MediaError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s %v", e.Op, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Op, e.Message)
}

func (e *MediaError) Unwarp() error {
	return e.Err
}

func ResizeJPEG(scrPath, dstPath string, width, quality int) error {
	if width > 2880 {
		return &MediaError{
			Op:      "Resize new image",
			Code:    400,
			Message: "Image resolution limit exceeded! Max width is 2880px",
			Err:     nil,
		}
	}

	file, err := os.Open(scrPath)
	if err != nil {
		return &MediaError{
			Op:      "Open filepath",
			Code:    404,
			Message: "File wasn't opened due to invalid name",
			Err:     err,
		}
	}
	defer file.Close()

	srcImg, _, err := image.Decode(file)
	if err != nil {
		return &MediaError{
			Op:      "Decoding file",
			Code:    400,
			Message: "The image may not be decodable due to damage or an incorrect structure",
			Err:     err,
		}
	}

	bounds := srcImg.Bounds()
	originalWidth := bounds.Dx()
	originalHeight := bounds.Dy()

	newHeight := (originalHeight * width) / originalWidth

	dstImg := image.NewRGBA(image.Rect(0, 0, width, newHeight))

	draw.BiLinear.Scale(dstImg, dstImg.Bounds(), srcImg, srcImg.Bounds(), draw.Over, nil)

	cacheFile, err := os.Create(dstPath)
	if err != nil {
		return &MediaError{
			Op:      "Creating cache file",
			Code:    500,
			Message: "Creating error",
			Err:     err,
		}
	}
	defer cacheFile.Close()

	options := &jpeg.Options{
		Quality: quality,
	}

	err = jpeg.Encode(cacheFile, dstImg, options)
	if err != nil {
		log.Printf("Encoding error %v\n", err)
		return &MediaError{
			Op:   "Encoding data into cache file",
			Code: 400,
			Err:  err,
		}
	}

	return nil
}

func Detector(Body io.Reader) (io.Reader, error) {
	buf := make([]byte, 512)

	_, err := io.ReadFull(Body, buf)
	if err != nil {
		return nil, &MediaError{
			Op:      "Reading file header",
			Code:    400,
			Message: "Failed to read file header",
			Err:     err,
		}
	}

	fileType := http.DetectContentType(buf)
	if fileType != "image/jpeg" && fileType != "image/png" {
		return nil, &MediaError{
			Op:      "Detecting file type",
			Code:    400,
			Message: "Incorrect file type!",
			Err:     nil,
		}
	}

	stream := io.MultiReader(bytes.NewReader(buf), Body)

	return stream, nil
}
