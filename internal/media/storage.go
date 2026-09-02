package media

import (
	"context"
	"io"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/wizzyszn/cooked/internal/config"
)

type ObjectInfo struct {
	Size        int64
	ContentType string
}

type ObjectStore interface {
	PresignPut(context.Context, string, string, time.Duration) (*url.URL, error)
	PresignGet(context.Context, string, time.Duration) (*url.URL, error)
	Stat(context.Context, string) (ObjectInfo, error)
	Get(context.Context, string) (io.ReadCloser, error)
	Put(context.Context, string, io.Reader, int64, string) error
	Delete(context.Context, string) error
}

type S3Store struct {
	client *minio.Client
	bucket string
}

func NewS3Store(cfg config.ObjectStorageConfig) (*S3Store, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{Creds: credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""), Secure: cfg.UseSSL, Region: cfg.Region})
	if err != nil {
		return nil, err
	}
	return &S3Store{client: client, bucket: cfg.Bucket}, nil
}

func (s *S3Store) PresignPut(ctx context.Context, key, contentType string, expiry time.Duration) (*url.URL, error) {
	return s.client.PresignedPutObject(ctx, s.bucket, key, expiry)
}
func (s *S3Store) PresignGet(ctx context.Context, key string, expiry time.Duration) (*url.URL, error) {
	return s.client.PresignedGetObject(ctx, s.bucket, key, expiry, nil)
}
func (s *S3Store) Stat(ctx context.Context, key string) (ObjectInfo, error) {
	info, err := s.client.StatObject(ctx, s.bucket, key, minio.StatObjectOptions{})
	return ObjectInfo{Size: info.Size, ContentType: info.ContentType}, err
}
func (s *S3Store) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	if _, err = obj.Stat(); err != nil {
		_ = obj.Close()
		return nil, err
	}
	return obj, nil
}
func (s *S3Store) Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error {
	_, err := s.client.PutObject(ctx, s.bucket, key, body, size, minio.PutObjectOptions{ContentType: contentType})
	return err
}
func (s *S3Store) Delete(ctx context.Context, key string) error {
	return s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
}
