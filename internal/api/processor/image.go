package processor

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/gif"
	_ "image/gif"
	"image/jpeg"
	_ "image/jpeg"
	"image/png"
	_ "image/png"
	"io"
	"log"
	"path/filepath"
	"strings"

	"github.com/finnsmithermay/mediaforge/internal/api/storage"
	"github.com/finnsmithermay/mediaforge/pkg/models"
	"github.com/nfnt/resize"
	_ "golang.org/x/image/webp"
)

const thumbnailMaxWidth = 320

// ImageProcessor resizes images and extracts metadata.
type ImageProcessor struct {
	storage storage.Client
}

// NewImageProcessor creates a new image processor.
func NewImageProcessor(storageClient storage.Client) *ImageProcessor {
	return &ImageProcessor{
		storage: storageClient,
	}
}

// Supports returns true for image media types.
func (p *ImageProcessor) Supports(mediaType models.MediaType) bool {
	return mediaType == models.MediaTypeImage
}

// Process resizes an image and extracts its metadata.
func (p *ImageProcessor) Process(ctx context.Context, job *models.Job) (*models.ProcessResult, error) {
	// Download the original image from S3
	s3Key := fmt.Sprintf("originals/%s%s", job.ID, filepath.Ext(job.FileName))

	body, err := p.storage.Download(ctx, s3Key)
	if err != nil {
		return nil, fmt.Errorf("downloading image: %w", err)
	}
	defer body.Close()

	// Read the image into memory (images are small enough for this)
	imgData, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("reading image data: %w", err)
	}

	// Decode the image
	img, format, err := image.Decode(bytes.NewReader(imgData))
	if err != nil {
		return nil, fmt.Errorf("decoding image: %w", err)
	}

	// Extract metadata
	bounds := img.Bounds()
	metadata := &models.Metadata{
		Width:  bounds.Dx(),
		Height: bounds.Dy(),
		Format: format,
		Size:   int64(len(imgData)),
	}
	log.Printf("Image metadata: %dx%d, format=%s, size=%d bytes",
		metadata.Width, metadata.Height, metadata.Format, metadata.Size)

	// Generate thumbnail
	thumbnail := resize.Resize(thumbnailMaxWidth, 0, img, resize.Lanczos3)
	// 0 for height means preserve aspect ratio

	// Encode thumbnail
	var thumbBuf bytes.Buffer
	if err := p.encodeThumbnail(&thumbBuf, thumbnail, format); err != nil {
		return nil, fmt.Errorf("encoding thumbnail: %w", err)
	}

	// Upload thumbnail to S3
	thumbExt := thumbnailExtension(format)
	thumbnailKey := fmt.Sprintf("thumbnails/%s%s", job.ID, thumbExt)

	_, err = p.storage.Upload(ctx, thumbnailKey, bytes.NewReader(thumbBuf.Bytes()), thumbnailContentType(format))
	if err != nil {
		return nil, fmt.Errorf("uploading thumbnail: %w", err)
	}

	// Get thumbnail URL
	thumbnailURL, err := p.storage.GetURL(ctx, thumbnailKey)
	if err != nil {
		return nil, fmt.Errorf("generating thumbnail URL: %w", err)
	}

	return &models.ProcessResult{
		ThumbnailURL: thumbnailURL,
		Metadata:     metadata,
	}, nil
}

func (p *ImageProcessor) encodeThumbnail(w io.Writer, img image.Image, format string) error {
	switch strings.ToLower(format) {
	case "jpeg":
		return jpeg.Encode(w, img, &jpeg.Options{Quality: 85})
	case "png":
		return png.Encode(w, img)
	case "gif":
		return gif.Encode(w, img, nil)
	default:
		// Default to JPEG for unsupported formats
		return jpeg.Encode(w, img, &jpeg.Options{Quality: 85})
	}
}

func thumbnailExtension(format string) string {
	switch strings.ToLower(format) {
	case "jpeg":
		return ".jpg"
	case "png":
		return ".png"
	case "gif":
		return ".gif"
	default:
		return ".jpg"
	}
}

func thumbnailContentType(format string) string {
	switch strings.ToLower(format) {
	case "jpeg":
		return "image/jpeg"
	case "png":
		return "image/png"
	case "gif":
		return "image/gif"
	default:
		return "image/jpeg"
	}
}
