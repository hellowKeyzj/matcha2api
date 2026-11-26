package repository

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type desktopIssueAttachmentStore struct {
	LocalMatchaPackageObjectStore
}

func ProvideDesktopIssueAttachmentStore() service.DesktopIssueAttachmentStore {
	dataDir := strings.TrimSpace(os.Getenv("DATA_DIR"))
	if dataDir == "" {
		dataDir = "data"
	}
	return &desktopIssueAttachmentStore{LocalMatchaPackageObjectStore{root: filepath.Join(dataDir, "desktop-issue-attachments")}}
}

func (s *desktopIssueAttachmentStore) Upload(ctx context.Context, key string, body io.Reader, contentType string) (int64, error) {
	if err := os.MkdirAll(s.root, 0o700); err != nil {
		return 0, err
	}
	return s.LocalMatchaPackageObjectStore.Upload(ctx, key, body, contentType)
}

func (s *desktopIssueAttachmentStore) Download(_ context.Context, key string) (service.DesktopIssueAttachmentFile, error) {
	filePath, err := s.path(key)
	if err != nil {
		return nil, err
	}
	return os.Open(filePath)
}
