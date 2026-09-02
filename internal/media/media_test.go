package media

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/wizzyszn/cooked/internal/domain"
	apperrors "github.com/wizzyszn/cooked/pkg/errors"
)

type fakeObjects struct {
	data       map[string][]byte
	deleted    []string
	getErrOnce bool
}

func (f *fakeObjects) PresignPut(_ context.Context, key, _ string, _ time.Duration) (*url.URL, error) {
	return url.Parse("https://objects.test/put/" + key)
}
func (f *fakeObjects) PresignGet(_ context.Context, key string, _ time.Duration) (*url.URL, error) {
	return url.Parse("https://objects.test/get/" + key)
}
func (f *fakeObjects) Stat(_ context.Context, key string) (ObjectInfo, error) {
	return ObjectInfo{Size: int64(len(f.data[key]))}, nil
}
func (f *fakeObjects) Get(_ context.Context, key string) (io.ReadCloser, error) {
	if f.getErrOnce {
		f.getErrOnce = false
		return nil, errors.New("temporary object-store outage")
	}
	return io.NopCloser(bytes.NewReader(f.data[key])), nil
}
func (f *fakeObjects) Put(_ context.Context, key string, r io.Reader, _ int64, _ string) error {
	b, _ := io.ReadAll(r)
	f.data[key] = b
	return nil
}
func (f *fakeObjects) Delete(_ context.Context, key string) error {
	f.deleted = append(f.deleted, key)
	delete(f.data, key)
	return nil
}

type fakeRepo struct {
	asset     *domain.MediaAsset
	failed    bool
	retry     bool
	completed bool
	variants  []domain.MediaVariant
	lastClaim domain.MediaAsset
}

func (f *fakeRepo) Create(_ context.Context, a *domain.MediaAsset) error { f.asset = a; return nil }
func (f *fakeRepo) Find(_ context.Context, id uuid.UUID) (*domain.MediaAsset, error) {
	if f.asset != nil && f.asset.ID == id {
		return f.asset, nil
	}
	return nil, nil
}
func (f *fakeRepo) MarkUploaded(_ context.Context, _ uuid.UUID, _ uuid.UUID, size int64, _ time.Time) error {
	f.asset.ProcessingStatus = domain.MediaUploaded
	f.asset.ByteSize = &size
	return nil
}
func (f *fakeRepo) Claim(_ context.Context, _ string, _ int, _ time.Time) ([]domain.MediaAsset, error) {
	if f.asset == nil {
		return nil, nil
	}
	a := *f.asset
	f.lastClaim = a
	f.asset = nil
	return []domain.MediaAsset{a}, nil
}
func (f *fakeRepo) Complete(_ context.Context, _ uuid.UUID, _ string, _ string, _ int64, _ int, _ int, v []domain.MediaVariant, _ time.Time) error {
	f.completed = true
	f.variants = v
	return nil
}
func (f *fakeRepo) Fail(_ context.Context, _ uuid.UUID, retry bool, _ string, _ time.Time) error {
	f.failed = true
	f.retry = retry
	if retry {
		a := f.lastClaim
		a.AttemptCount++
		f.asset = &a
	}
	return nil
}
func (f *fakeRepo) ClaimOrphans(context.Context, int, time.Time) ([]domain.MediaAsset, error) {
	return nil, nil
}
func (f *fakeRepo) MarkDeleted(context.Context, uuid.UUID) error { return nil }

func TestInitializeRejectsOversizedUpload(t *testing.T) {
	s := NewService(&fakeRepo{}, &fakeObjects{data: map[string][]byte{}}, nil)
	_, err := s.Initialize(t.Context(), uuid.New(), InitializeRequest{Purpose: domain.MediaPurposeProfileAvatar, MIMEType: "image/jpeg", ByteSize: maxUploadBytes + 1, AccessScope: domain.MediaPublic})
	if err == nil {
		t.Fatal("expected oversized upload rejection")
	}
}

func TestCompleteRejectsOversizedObject(t *testing.T) {
	owner, id := uuid.New(), uuid.New()
	declared := int64(12)
	repo := &fakeRepo{asset: &domain.MediaAsset{BaseModel: domain.BaseModel{ID: id}, OwnerID: &owner, ObjectKey: "original/large.jpg", ByteSize: &declared, ProcessingStatus: domain.MediaAwaitingUpload, ExpiresAt: time.Now().Add(time.Minute)}}
	objects := &fakeObjects{data: map[string][]byte{"original/large.jpg": make([]byte, maxUploadBytes+1)}}
	if _, err := NewService(repo, objects, nil).CompleteUpload(t.Context(), owner, id); err == nil {
		t.Fatal("expected oversized object rejection")
	}
	if !repo.failed || len(objects.deleted) != 1 {
		t.Fatalf("oversized object was not quarantined and deleted: repo=%+v deleted=%v", repo, objects.deleted)
	}
}
func TestGetQuarantinesPendingAndPrivateAssets(t *testing.T) {
	owner := uuid.New()
	id := uuid.New()
	repo := &fakeRepo{asset: &domain.MediaAsset{BaseModel: domain.BaseModel{ID: id}, OwnerID: &owner, ProcessingStatus: domain.MediaUploaded, ModerationStatus: domain.MediaModerationPending, AccessScope: domain.MediaPublic}}
	s := NewService(repo, &fakeObjects{data: map[string][]byte{}}, nil)
	if _, err := s.Get(t.Context(), nil, id); err != apperrors.ErrNotFound {
		t.Fatalf("public pending error=%v", err)
	}
	if _, err := s.Get(t.Context(), &owner, id); err != nil {
		t.Fatalf("owner pending access: %v", err)
	}
	repo.asset.ProcessingStatus = domain.MediaReady
	repo.asset.ModerationStatus = domain.MediaModerationApproved
	repo.asset.AccessScope = domain.MediaPrivate
	if _, err := s.Get(t.Context(), nil, id); err != apperrors.ErrNotFound {
		t.Fatalf("private public error=%v", err)
	}
	repo.asset.OwnerID = nil
	if _, err := s.Get(t.Context(), &owner, id); err != apperrors.ErrNotFound {
		t.Fatalf("deleted owner private error=%v", err)
	}
}
func TestProcessorRejectsSpoofedMIME(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	var b bytes.Buffer
	_ = png.Encode(&b, img)
	id := uuid.New()
	key := "original/test.png"
	repo := &fakeRepo{asset: &domain.MediaAsset{BaseModel: domain.BaseModel{ID: id}, ObjectKey: key, DeclaredMIMEType: "image/jpeg"}}
	p := NewProcessor(repo, &fakeObjects{data: map[string][]byte{key: b.Bytes()}}, "worker", 1, nil)
	if err := p.RunOnce(t.Context()); err != nil {
		t.Fatal(err)
	}
	if !repo.failed || repo.retry {
		t.Fatalf("spoofed image should be permanently quarantined: %+v", repo)
	}
}
func TestProcessorStripsMetadataAndCreatesVariants(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1200, 600))
	for y := 0; y < 600; y++ {
		for x := 0; x < 1200; x++ {
			img.Set(x, y, color.RGBA{R: 100, G: 50, B: 20, A: 255})
		}
	}
	var b bytes.Buffer
	_ = jpeg.Encode(&b, img, &jpeg.Options{Quality: 90})
	id := uuid.New()
	key := "original/test.jpg"
	objects := &fakeObjects{data: map[string][]byte{key: b.Bytes()}}
	repo := &fakeRepo{asset: &domain.MediaAsset{BaseModel: domain.BaseModel{ID: id}, ObjectKey: key, DeclaredMIMEType: "image/jpeg"}}
	if err := NewProcessor(repo, objects, "worker", 1, nil).RunOnce(t.Context()); err != nil {
		t.Fatal(err)
	}
	if !repo.completed || len(repo.variants) != 2 {
		t.Fatalf("completed=%t variants=%d", repo.completed, len(repo.variants))
	}
	if repo.variants[0].Width > 256 || repo.variants[1].Width > 1024 {
		t.Fatalf("invalid variants: %+v", repo.variants)
	}
}

func TestMediaJobSurvivesWorkerRestart(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	var b bytes.Buffer
	_ = jpeg.Encode(&b, img, nil)
	id, key := uuid.New(), "original/restart.jpg"
	repo := &fakeRepo{asset: &domain.MediaAsset{BaseModel: domain.BaseModel{ID: id}, ObjectKey: key, DeclaredMIMEType: "image/jpeg"}}
	objects := &fakeObjects{data: map[string][]byte{key: b.Bytes()}, getErrOnce: true}
	if err := NewProcessor(repo, objects, "worker-a", 1, nil).RunOnce(t.Context()); err != nil {
		t.Fatal(err)
	}
	if !repo.retry || repo.asset == nil {
		t.Fatal("transient failure was not left retryable")
	}
	if err := NewProcessor(repo, objects, "worker-b", 1, nil).RunOnce(t.Context()); err != nil {
		t.Fatal(err)
	}
	if !repo.completed {
		t.Fatal("replacement worker did not complete the persisted job")
	}
}

func TestValidateProfileAvatarRequiresReadyOwnedAsset(t *testing.T) {
	owner, id := uuid.New(), uuid.New()
	repo := &fakeRepo{asset: &domain.MediaAsset{BaseModel: domain.BaseModel{ID: id}, OwnerID: &owner, Purpose: domain.MediaPurposeProfileAvatar, ProcessingStatus: domain.MediaReady, ModerationStatus: domain.MediaModerationApproved, AccessScope: domain.MediaPublic}}
	if err := NewService(repo, &fakeObjects{}, nil).ValidateProfileAvatar(t.Context(), owner, id); err != nil {
		t.Fatalf("valid avatar: %v", err)
	}
	other := uuid.New()
	if err := NewService(repo, &fakeObjects{}, nil).ValidateProfileAvatar(t.Context(), other, id); err == nil {
		t.Fatal("another user's avatar was accepted")
	}
}
