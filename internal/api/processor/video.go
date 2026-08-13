package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/finnsmithermay/mediaforge/internal/api/storage"
	"github.com/finnsmithermay/mediaforge/pkg/models"
)

// VideoProcessor extracts thumbnails and metadata from video files.
type VideoProcessor struct {
	storage storage.Client
}

// NewVideoProcessor creates a new video processor.
func NewVideoProcessor(storageClient storage.Client) *VideoProcessor {
	return &VideoProcessor{
		storage: storageClient,
	}
}

// Supports returns true for video media types.
func (p *VideoProcessor) Supports(mediaType models.MediaType) bool {
	return mediaType == models.MediaTypeVideo
}

// ffprobeOutput matches the JSON structure that ffprobe returns.
type ffprobeOutput struct {
	Streams []ffprobeStream `json:"streams"`
	Format  ffprobeFormat   `json:"format"`
}

type ffprobeStream struct {
	CodecName string `json:"codec_name"`
	CodecType string `json:"codec_type"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
}

type ffprobeFormat struct {
	Duration string `json:"duration"`
	Size     string `json:"size"`
}

func (p *VideoProcessor) extractMetadata(ctx context.Context, videoPath string) (*models.Metadata, error) {
	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		videoPath,
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("running ffprobe: %w", err)
	}

	var probe ffprobeOutput
	if err := json.Unmarshal(output, &probe); err != nil {
		return nil, fmt.Errorf("parsing ffprobe output: %w", err)
	}

	metadata := &models.Metadata{}

	// Find the video stream
	for _, stream := range probe.Streams {
		if stream.CodecType == "video" {
			metadata.Width = stream.Width
			metadata.Height = stream.Height
			metadata.Codec = stream.CodecName
			break
		}
	}

	// Parse duration
	if probe.Format.Duration != "" {
		duration, err := strconv.ParseFloat(probe.Format.Duration, 64)
		if err == nil {
			metadata.Duration = duration
		}
	}

	// Parse file size
	if probe.Format.Size != "" {
		size, err := strconv.ParseInt(probe.Format.Size, 10, 64)
		if err == nil {
			metadata.Size = size
		}
	}

	metadata.Format = "video"

	return metadata, nil
}

func (p *VideoProcessor) extractThumbnail(ctx context.Context, videoPath, outputPath string) error {
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-i", videoPath,
		"-ss", "00:00:01", // seek to 1 second
		"-vframes", "1", // extract 1 frame
		"-q:v", "2", // high quality JPEG
		"-y", // overwrite output file
		outputPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("running ffmpeg: %w (output: %s)", err, string(output))
	}

	return nil
}

// Process extracts a thumbnail and metadata from a video file.
func (p *VideoProcessor) Process(ctx context.Context, job *models.Job) (*models.ProcessResult, error) {
	// Create a temp directory for this job's working files
	tmpDir, err := os.MkdirTemp("", "mediaforge-video-*")
	if err != nil {
		return nil, fmt.Errorf("creating temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Download the original video from S3
	videoPath := filepath.Join(tmpDir, "input"+filepath.Ext(job.FileName))
	if err := p.downloadToFile(ctx, job, videoPath); err != nil {
		return nil, fmt.Errorf("downloading video: %w", err)
	}

	// Extract metadata
	metadata, err := p.extractMetadata(ctx, videoPath)
	if err != nil {
		return nil, fmt.Errorf("extracting metadata: %w", err)
	}
	log.Printf("Video metadata: %dx%d, %.1fs, codec=%s", metadata.Width, metadata.Height, metadata.Duration, metadata.Codec)

	// Extract thumbnail
	thumbnailPath := filepath.Join(tmpDir, "thumbnail.jpg")
	if err := p.extractThumbnail(ctx, videoPath, thumbnailPath); err != nil {
		return nil, fmt.Errorf("extracting thumbnail: %w", err)
	}

	// Upload thumbnail to S3
	thumbnailKey := fmt.Sprintf("thumbnails/%s.jpg", job.ID)
	thumbnailFile, err := os.Open(thumbnailPath)
	if err != nil {
		return nil, fmt.Errorf("opening thumbnail: %w", err)
	}
	defer thumbnailFile.Close()

	_, err = p.storage.Upload(ctx, thumbnailKey, thumbnailFile, "image/jpeg")
	if err != nil {
		return nil, fmt.Errorf("uploading thumbnail: %w", err)
	}

	// Get the thumbnail URL
	thumbnailURL, err := p.storage.GetURL(ctx, thumbnailKey)
	if err != nil {
		return nil, fmt.Errorf("generating thumbnail URL: %w", err)
	}

	return &models.ProcessResult{
		ThumbnailURL: thumbnailURL,
		Metadata:     metadata,
	}, nil
}

func (p *VideoProcessor) downloadToFile(ctx context.Context, job *models.Job, destPath string) error {
	s3Key := fmt.Sprintf("originals/%s%s", job.ID, filepath.Ext(job.FileName))

	body, err := p.storage.Download(ctx, s3Key)
	if err != nil {
		return err
	}
	defer body.Close()

	outFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("creating file: %w", err)
	}
	defer outFile.Close()

	if _, err := outFile.ReadFrom(body); err != nil {
		return fmt.Errorf("writing file: %w", err)
	}

	return nil
}
