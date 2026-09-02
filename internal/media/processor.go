package media

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/wizzyszn/cooked/internal/domain"
	"go.uber.org/zap"
)

const maxDimension = 12000

type invalidImageError struct{ message string }

func (e invalidImageError) Error() string { return e.message }

type ProcessorRepository interface {
	Claim(context.Context, string, int, time.Time) ([]domain.MediaAsset, error)
	Complete(context.Context, uuid.UUID, string, string, int64, int, int, []domain.MediaVariant, time.Time) error
	Fail(context.Context, uuid.UUID, bool, string, time.Time) error
	ClaimOrphans(context.Context, int, time.Time) ([]domain.MediaAsset, error)
	MarkDeleted(context.Context, uuid.UUID) error
}

type Processor struct {
	repo      ProcessorRepository
	objects   ObjectStore
	workerID  string
	batchSize int
	log       *zap.SugaredLogger
	now       func() time.Time
}

func NewProcessor(repo ProcessorRepository, objects ObjectStore, workerID string, batchSize int, log *zap.SugaredLogger) *Processor {
	if batchSize <= 0 {
		batchSize = 20
	}
	return &Processor{repo: repo, objects: objects, workerID: workerID, batchSize: batchSize, log: log, now: func() time.Time { return time.Now().UTC() }}
}
func (p *Processor) RunOnce(ctx context.Context) error {
	now := p.now()
	assets, err := p.repo.Claim(ctx, p.workerID, p.batchSize, now)
	if err != nil {
		return err
	}
	for i := range assets {
		if err = p.process(ctx, &assets[i]); err != nil {
			var invalid invalidImageError
			retry := !errors.As(err, &invalid) && assets[i].AttemptCount+1 < 5
			delay := time.Duration(1<<min(assets[i].AttemptCount, 6)) * time.Minute
			_ = p.repo.Fail(ctx, assets[i].ID, retry, err.Error(), now.Add(delay))
			if p.log != nil {
				p.log.Warnw("media processing failed", "asset_id", assets[i].ID, "retry", retry, "error", err)
			}
		}
	}
	orphans, e := p.repo.ClaimOrphans(ctx, p.batchSize, now)
	if e != nil {
		return e
	}
	for i := range orphans {
		_ = p.objects.Delete(ctx, orphans[i].ObjectKey)
		_ = p.repo.MarkDeleted(ctx, orphans[i].ID)
	}
	return nil
}
func (p *Processor) process(ctx context.Context, a *domain.MediaAsset) error {
	r, err := p.objects.Get(ctx, a.ObjectKey)
	if err != nil {
		return fmt.Errorf("download original: %w", err)
	}
	defer r.Close()
	raw, err := io.ReadAll(io.LimitReader(r, maxUploadBytes+1))
	if err != nil {
		return fmt.Errorf("read original: %w", err)
	}
	if int64(len(raw)) > maxUploadBytes {
		return invalidImageError{"image exceeds 5 MB"}
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return invalidImageError{"content is not a supported image"}
	}
	mime := formatMIME(format)
	if mime == "" || mime != a.DeclaredMIMEType {
		return invalidImageError{"decoded MIME type does not match the declared type"}
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > maxDimension || cfg.Height > maxDimension || int64(cfg.Width)*int64(cfg.Height) > 40_000_000 {
		return invalidImageError{"image dimensions are unsafe"}
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return invalidImageError{"image cannot be decoded"}
	}
	normalized, err := encodeJPEG(img, 90)
	if err != nil {
		return fmt.Errorf("normalize image: %w", err)
	}
	if err = p.objects.Put(ctx, a.ObjectKey, bytes.NewReader(normalized), int64(len(normalized)), "image/jpeg"); err != nil {
		return fmt.Errorf("store normalized image: %w", err)
	}
	variants := make([]domain.MediaVariant, 0, 2)
	for _, spec := range []struct {
		name string
		max  int
	}{{"thumbnail", 256}, {"medium", 1024}} {
		resized, w, h := resizeFit(img, spec.max)
		encoded, e := encodeJPEG(resized, 85)
		if e != nil {
			return e
		}
		key := fmt.Sprintf("variants/%s/%s.jpg", a.ID, spec.name)
		if e = p.objects.Put(ctx, key, bytes.NewReader(encoded), int64(len(encoded)), "image/jpeg"); e != nil {
			return fmt.Errorf("store %s: %w", spec.name, e)
		}
		variants = append(variants, domain.MediaVariant{Name: spec.name, ObjectKey: key, MIMEType: "image/jpeg", ByteSize: int64(len(encoded)), Width: w, Height: h})
	}
	sum := sha256.Sum256(normalized)
	return p.repo.Complete(ctx, a.ID, "image/jpeg", hex.EncodeToString(sum[:]), int64(len(normalized)), cfg.Width, cfg.Height, variants, p.now())
}
func formatMIME(format string) string {
	switch format {
	case "jpeg":
		return "image/jpeg"
	case "png":
		return "image/png"
	}
	return ""
}
func encodeJPEG(img image.Image, quality int) ([]byte, error) {
	var b bytes.Buffer
	err := jpeg.Encode(&b, img, &jpeg.Options{Quality: quality})
	return b.Bytes(), err
}
func resizeFit(src image.Image, maxSize int) (image.Image, int, int) {
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	if sw <= maxSize && sh <= maxSize {
		return src, sw, sh
	}
	scale := float64(maxSize) / float64(max(sw, sh))
	dw, dh := max(1, int(float64(sw)*scale)), max(1, int(float64(sh)*scale))
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < dh; y++ {
		sy := b.Min.Y + y*sh/dh
		for x := 0; x < dw; x++ {
			sx := b.Min.X + x*sw/dw
			dst.Set(x, y, src.At(sx, sy))
		}
	}
	return dst, dw, dh
}
