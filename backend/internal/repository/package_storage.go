package repository

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/servertiming"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type LocalMatchaPackageObjectStore struct {
	root string
}

type S3MatchaPackageObjectStore struct {
	client        *s3.Client
	bucket        string
	prefix        string
	publicBaseURL string
	presignExpiry time.Duration
}

var _ service.MatchaPackageObjectStore = (*LocalMatchaPackageObjectStore)(nil)
var _ service.MatchaPackageObjectStore = (*S3MatchaPackageObjectStore)(nil)

func NewLocalMatchaPackageObjectStore(root string) service.MatchaPackageObjectStore {
	root = strings.TrimSpace(root)
	if root == "" {
		root = localMatchaPackageObjectRoot()
	}
	return &LocalMatchaPackageObjectStore{root: root}
}

func ProvideMatchaPackageObjectStoreFactory() service.MatchaPackageObjectStoreFactory {
	return func(ctx context.Context, cfg *service.MatchaPackageStorageConfig) (service.MatchaPackageObjectStore, error) {
		if cfg == nil || cfg.Driver == "" || cfg.Driver == "local" {
			root := ""
			if cfg != nil {
				root = cfg.BasePath
			}
			return NewLocalMatchaPackageObjectStore(root), nil
		}
		return NewS3MatchaPackageObjectStore(ctx, cfg)
	}
}

func (s *LocalMatchaPackageObjectStore) Upload(_ context.Context, key string, body io.Reader, contentType string) (int64, error) {
	_ = contentType
	path, err := s.path(key)
	if err != nil {
		return 0, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return 0, err
	}
	file, err := os.Create(path)
	if err != nil {
		return 0, err
	}
	defer func() { _ = file.Close() }()
	return io.Copy(file, body)
}

func (s *LocalMatchaPackageObjectStore) Download(_ context.Context, key string) (io.ReadCloser, error) {
	path, err := s.path(key)
	if err != nil {
		return nil, err
	}
	return os.Open(path)
}

func (s *LocalMatchaPackageObjectStore) Delete(_ context.Context, key string) error {
	path, err := s.path(key)
	if err != nil {
		return err
	}
	return os.Remove(path)
}

func (s *LocalMatchaPackageObjectStore) PresignURL(_ context.Context, key string) (string, error) {
	path, err := s.path(key)
	if err != nil {
		return "", err
	}
	return "file://" + filepath.ToSlash(path), nil
}

func (s *LocalMatchaPackageObjectStore) path(key string) (string, error) {
	key = filepath.Clean(strings.TrimSpace(filepath.FromSlash(key)))
	key = strings.TrimLeft(key, string(filepath.Separator))
	if key == "." || key == "" || strings.HasPrefix(key, "..") || filepath.IsAbs(key) {
		return "", infraerrors.BadRequest("INVALID_MATCHA_PACKAGE_OBJECT_KEY", "invalid package object key")
	}
	return filepath.Join(s.root, key), nil
}

func localMatchaPackageObjectRoot() string {
	base := strings.TrimSpace(os.Getenv("DATA_DIR"))
	if base == "" {
		base = "data"
	}
	return filepath.Join(base, "matcha-packages")
}

func NewS3MatchaPackageObjectStore(ctx context.Context, cfg *service.MatchaPackageStorageConfig) (*S3MatchaPackageObjectStore, error) {
	client, err := newS3Client(ctx, s3ClientParams{
		Endpoint:        cfg.Endpoint,
		Region:          cfg.Region,
		AccessKeyID:     cfg.AccessKeyID,
		SecretAccessKey: cfg.SecretAccessKey,
		ForcePathStyle:  cfg.ForcePathStyle,
	})
	if err != nil {
		return nil, err
	}
	expiry := time.Duration(cfg.PresignExpiryHours) * time.Hour
	if expiry <= 0 {
		expiry = 24 * time.Hour
	}
	return &S3MatchaPackageObjectStore{
		client:        client,
		bucket:        cfg.Bucket,
		prefix:        strings.Trim(strings.TrimSpace(cfg.Prefix), "/"),
		publicBaseURL: strings.TrimRight(cfg.PublicBaseURL, "/"),
		presignExpiry: expiry,
	}, nil
}

func (s *S3MatchaPackageObjectStore) Upload(ctx context.Context, key string, body io.Reader, contentType string) (int64, error) {
	key = s.key(key)
	counter := &countingReader{r: body}
	finish := servertiming.ObserveDependency(ctx, "s3")
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      &s.bucket,
		Key:         &key,
		Body:        counter,
		ContentType: &contentType,
	})
	finish()
	if err != nil {
		return 0, fmt.Errorf("S3 PutObject: %w", err)
	}
	return counter.n, nil
}

func (s *S3MatchaPackageObjectStore) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	key = s.key(key)
	finish := servertiming.ObserveDependency(ctx, "s3")
	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &s.bucket,
		Key:    &key,
	})
	finish()
	if err != nil {
		return nil, fmt.Errorf("S3 GetObject: %w", err)
	}
	return result.Body, nil
}

func (s *S3MatchaPackageObjectStore) Delete(ctx context.Context, key string) error {
	key = s.key(key)
	finish := servertiming.ObserveDependency(ctx, "s3")
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &s.bucket,
		Key:    &key,
	})
	finish()
	return err
}

func (s *S3MatchaPackageObjectStore) PresignURL(ctx context.Context, key string) (string, error) {
	key = s.key(key)
	if s.publicBaseURL != "" {
		return s.publicBaseURL + "/" + strings.TrimLeft(key, "/"), nil
	}
	presignClient := s3.NewPresignClient(s.client)
	result, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: &s.bucket,
		Key:    &key,
	}, s3.WithPresignExpires(s.presignExpiry))
	if err != nil {
		return "", fmt.Errorf("presign url: %w", err)
	}
	return result.URL, nil
}

func (s *S3MatchaPackageObjectStore) key(key string) string {
	key = strings.TrimLeft(strings.TrimSpace(key), "/")
	if s.prefix == "" {
		return key
	}
	return s.prefix + "/" + key
}

type countingReader struct {
	r io.Reader
	n int64
}

func (r *countingReader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	r.n += int64(n)
	return n, err
}
