package core

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/jpeg"
	"image/png"
	_ "image/png"
	"io"
	"net/http"

	"github.com/chai2010/webp"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
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

func (e *MediaError) Unwrap() error {
	return e.Err
}

func ResizeImage(ctx context.Context, scrPath, object, OrigBucket string, width, quality int, minioClient *minio.Client, format string) ([]byte, string, error) {
	if width > 2880 {
		return nil, "", &MediaError{
			Op:      "Resize new image",
			Code:    400,
			Message: "Image resolution limit exceeded! Max width is 2880px",
			Err:     nil,
		}
	}

	file, err := minioClient.GetObject(ctx, OrigBucket, object, minio.GetObjectOptions{})
	if err != nil {
		return nil, "", &MediaError{
			Op:      "Data transfer",
			Code:    500,
			Message: "Unexpected error, tell your REQ-ID to our support",
			Err:     err,
		}
	}
	defer file.Close()

	srcImg, _, err := image.Decode(file)
	if err != nil {
		return nil, "", &MediaError{
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

	jpegOptions := &jpeg.Options{
		Quality: quality,
	}

	webpOptions := &webp.Options{
		Quality: float32(quality),
	}

	var buf bytes.Buffer

	switch format {
	case "jpeg":
		err = jpeg.Encode(&buf, dstImg, jpegOptions) // writes data in buffer from dstImg
		if err != nil {
			return nil, "", &MediaError{
				Op:   "Encoding data into cache file",
				Code: 400,
				Err:  err,
			}
		}
	case "png":
		err = png.Encode(&buf, dstImg) // writes data in buffer from dstImg
		if err != nil {
			return nil, "", &MediaError{
				Op:      "Encoding data into cache file",
				Code:    400,
				Message: "Unexpected error, tell your REQ-ID to our support",
				Err:     err,
			}
		}
	case "webp":
		err = webp.Encode(&buf, dstImg, webpOptions) // writes data in buffer from dstImg
		if err != nil {
			return nil, "", &MediaError{
				Op:      "Encoding data into cache file",
				Code:    400,
				Message: "Unexpected error, tell your REQ-ID to our support",
				Err:     err,
			}
		}
	default:
		return nil, "", &MediaError{
			Op:      "Encoding data",
			Code:    400,
			Message: "Unexpected error, tell your REQ-ID to our support",
			Err:     nil,
		}
	}

	return buf.Bytes(), format, nil
}

func Detector(Body io.Reader) (io.Reader, string, error) {
	buf := make([]byte, 512)

	_, err := io.ReadFull(Body, buf)
	if err != nil {
		return nil, "", &MediaError{
			Op:      "Reading file header",
			Code:    400,
			Message: "Failed to read file header",
			Err:     err,
		}
	}

	fileType := http.DetectContentType(buf)
	if fileType != "image/jpeg" && fileType != "image/png" && fileType != "image/webp" {
		return nil, "", &MediaError{
			Op:      "Detecting file type",
			Code:    400,
			Message: "Incorrect file type!",
			Err:     nil,
		}
	}
	stream := io.MultiReader(bytes.NewReader(buf), Body)

	return stream, fileType, nil
}

func InitMinIO(ctx context.Context, CachedBucket, OriginalBucket string, endpoint string, accessKeyID string, secretAccessKey string, useSec bool) (*minio.Client, error) {

	minioClient, err := minio.New(endpoint, &minio.Options{
		Secure: useSec,
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
	})

	if err != nil {
		return nil, &MediaError{
			Op:      "Creating new minIO client",
			Code:    500,
			Message: "ERR, tell your REQ-ID code to our support",
			Err:     err,
		}
	}

	bucketOpts := minio.MakeBucketOptions{}

	res, err := minioClient.BucketExists(ctx, CachedBucket)
	if err != nil {
		return nil, &MediaError{
			Op:      "Creating new minIO buckets [cached]",
			Code:    500,
			Message: "ERR, tell your REQ-ID code to our support",
			Err:     err,
		}
	}
	if res == false {
		minioClient.MakeBucket(ctx, CachedBucket, bucketOpts)
	}

	res, err = minioClient.BucketExists(ctx, OriginalBucket)
	if err != nil {
		return nil, &MediaError{
			Op:      "Creating new minIO buckets [originals]",
			Code:    500,
			Message: "ERR, tell your REQ-ID code to our support",
			Err:     err,
		}
	}
	if res == false {
		minioClient.MakeBucket(ctx, OriginalBucket, minio.MakeBucketOptions{})
	}
	return minioClient, nil
}
