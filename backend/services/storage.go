package services

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"living-recorder/backend/config"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type FileInfo struct {
	Path  string
	Size  int64
	IsDir bool
}

type StorageBackend interface {
	Save(ctx context.Context, srcPath, destPath string) error
	Delete(ctx context.Context, path string) error
	List(ctx context.Context, prefix string) ([]FileInfo, error)
	GetURL(ctx context.Context, path string) (string, error)
}

type LocalStorage struct {
	basePath string
}

func NewLocalStorage(cfg config.LocalStorageConfig) *LocalStorage {
	return &LocalStorage{basePath: cfg.Path}
}

func (s *LocalStorage) Save(_ context.Context, srcPath, destPath string) error {
	fullPath := filepath.Join(s.basePath, destPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}

	srcFile, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("create dest: %w", err)
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return fmt.Errorf("copy: %w", err)
	}
	return dstFile.Close()
}

func (s *LocalStorage) Delete(_ context.Context, path string) error {
	return os.Remove(filepath.Join(s.basePath, path))
}

func (s *LocalStorage) List(_ context.Context, prefix string) ([]FileInfo, error) {
	entries, err := os.ReadDir(filepath.Join(s.basePath, prefix))
	if err != nil {
		return nil, err
	}
	infos := make([]FileInfo, 0, len(entries))
	for _, e := range entries {
		info, _ := e.Info()
		size := int64(0)
		if info != nil {
			size = info.Size()
		}
		infos = append(infos, FileInfo{
			Path:  filepath.Join(prefix, e.Name()),
			Size:  size,
			IsDir: e.IsDir(),
		})
	}
	return infos, nil
}

func (s *LocalStorage) GetURL(_ context.Context, path string) (string, error) {
	return filepath.Join(s.basePath, path), nil
}

type S3Storage struct {
	client *minio.Client
	bucket string
}

func NewS3Storage(cfg config.S3StorageConfig) (*S3Storage, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: false,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, err
	}
	return &S3Storage{client: client, bucket: cfg.Bucket}, nil
}

func (s *S3Storage) Save(ctx context.Context, srcPath, destPath string) error {
	if _, err := s.client.FPutObject(ctx, s.bucket, destPath, srcPath, minio.PutObjectOptions{}); err != nil {
		return fmt.Errorf("s3 save: %w", err)
	}
	return nil
}

func (s *S3Storage) Delete(ctx context.Context, path string) error {
	if err := s.client.RemoveObject(ctx, s.bucket, path, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("s3 delete: %w", err)
	}
	return nil
}

func (s *S3Storage) List(ctx context.Context, prefix string) ([]FileInfo, error) {
	objects := s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{Prefix: prefix})
	infos := make([]FileInfo, 0)
	for obj := range objects {
		if obj.Err != nil {
			return nil, obj.Err
		}
		infos = append(infos, FileInfo{
			Path:  obj.Key,
			Size:  obj.Size,
			IsDir: len(obj.Key) > 0 && obj.Key[len(obj.Key)-1] == '/',
		})
	}
	return infos, nil
}

func (s *S3Storage) GetURL(ctx context.Context, path string) (string, error) {
	u, err := s.client.PresignedGetObject(ctx, s.bucket, path, 86400, nil)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}
