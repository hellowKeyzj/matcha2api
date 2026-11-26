package handler_test

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/model"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const feedbackSubmitPath = "/api/v1/feedback/issues"

func feedbackSubmitRouter(svc *service.DesktopIssueService) *gin.Engine {
	router := gin.New()
	router.POST(feedbackSubmitPath, func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 1})
		c.Next()
	}, handler.NewDesktopIssueHandler(svc).Submit)
	return router
}

func TestDesktopIssueUploadHonorsTighterIngressLimit(t *testing.T) {
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	require.NoError(t, form.WriteField("title", strings.Repeat("x", 1024)))
	require.NoError(t, form.Close())
	req := httptest.NewRequest(http.MethodPost, feedbackSubmitPath, &body)
	req.Header.Set("Content-Type", form.FormDataContentType())
	w := httptest.NewRecorder()
	ingress := http.MaxBytesHandler(feedbackSubmitRouter(new(service.DesktopIssueService)), 512)
	ingress.ServeHTTP(w, req)

	require.Equal(t, http.StatusRequestEntityTooLarge, w.Code, w.Body.String())
}

func TestDesktopIssueUploadRequestLimitIncludesMultipartEpilogue(t *testing.T) {
	const limit = int64(256 << 20)
	for _, extra := range []int64{0, 1} {
		var formBody bytes.Buffer
		form := multipart.NewWriter(&formBody)
		require.NoError(t, form.WriteField("title", "Feedback"))
		require.NoError(t, form.Close())
		body := io.MultiReader(&formBody, io.LimitReader(feedbackZeroReader{}, limit-int64(formBody.Len())+extra))
		req := httptest.NewRequest(http.MethodPost, feedbackSubmitPath, body)
		req.Header.Set("Content-Type", form.FormDataContentType())
		require.EqualValues(t, -1, req.ContentLength)
		w := httptest.NewRecorder()
		svc := service.NewDesktopIssueService(&feedbackIssueRepo{}, nil, nil)
		feedbackSubmitRouter(svc).ServeHTTP(w, req)
		want := http.StatusCreated
		if extra > 0 {
			want = http.StatusRequestEntityTooLarge
		}
		require.Equal(t, want, w.Code, w.Body.String())
	}
}

func TestDesktopIssueUploadAttachmentLimits(t *testing.T) {
	feedbackTestDirs(t)
	const imageLimit, videoLimit, diagnosticsLimit = 10 << 20, 50 << 20, 50 << 20
	image := feedbackSizedUpload(t, "screen.png", "\x89PNG\r\n\x1a\n", imageLimit, 0)
	video := feedbackSizedUpload(t, "capture.webm", "\x1a\x45\xdf\xa3", videoLimit, 0)
	archive := string(feedbackDiagnosticsZIP(t))
	diagnostics := feedbackSizedUpload(t, "report.zip", archive, diagnosticsLimit, diagnosticsLimit-int64(len(archive)))
	extraImage, extraVideo, extraDiagnostics := image, video, diagnostics
	extraImage.SizeBytes++
	extraVideo.SizeBytes++
	extraDiagnostics.SizeBytes++
	mediumVideo := video
	mediumVideo.SizeBytes = 30 << 20
	overAggregateVideo := mediumVideo
	overAggregateVideo.SizeBytes++

	for _, tc := range []struct {
		name, reason string
		status       int
		media        []service.DesktopIssueAttachmentUpload
		diagnostics  *service.DesktopIssueAttachmentUpload
	}{
		{"all_limits_inclusive", "", http.StatusOK, []service.DesktopIssueAttachmentUpload{video, video, video, mediumVideo, image, image}, &diagnostics},
		{"image_over_limit", "IMAGE_TOO_LARGE", http.StatusRequestEntityTooLarge, []service.DesktopIssueAttachmentUpload{extraImage}, nil},
		{"video_over_limit", "VIDEO_TOO_LARGE", http.StatusRequestEntityTooLarge, []service.DesktopIssueAttachmentUpload{extraVideo}, nil},
		{"diagnostics_over_limit", "DIAGNOSTICS_TOO_LARGE", http.StatusRequestEntityTooLarge, nil, &extraDiagnostics},
		{"media_count_over_limit", "TOO_MANY_ATTACHMENTS", http.StatusBadRequest, []service.DesktopIssueAttachmentUpload{image, image, image, image, image, image, image}, nil},
		{"aggregate_over_limit", "ATTACHMENTS_TOO_LARGE", http.StatusRequestEntityTooLarge, []service.DesktopIssueAttachmentUpload{video, video, video, overAggregateVideo, image, image}, &diagnostics},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &feedbackIssueRepo{}
			store := feedbackDiscardStore{repository.ProvideDesktopIssueAttachmentStore()}
			svc := service.NewDesktopIssueService(repo, nil, store)
			detail, err := svc.Submit(t.Context(), service.SubmitDesktopIssueRequest{
				ReporterUserID: 1,
				Title:          "Attachment limits",
				Media:          tc.media,
				Diagnostics:    tc.diagnostics,
			})
			require.Equal(t, tc.status, infraerrors.Code(err), "%v", err)
			require.Equal(t, tc.reason, infraerrors.Reason(err))
			if err == nil {
				require.Len(t, detail.Attachments, 7)
				require.True(t, detail.Issue.HasDiagnostics)
			} else {
				require.Nil(t, repo.detail)
			}
		})
	}
}

func feedbackSizedUpload(t *testing.T, name, data string, size, offset int64) service.DesktopIssueAttachmentUpload {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "payload-")
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })
	require.NoError(t, f.Truncate(size+1))
	_, err = f.WriteAt([]byte(data), offset)
	require.NoError(t, err)
	return service.DesktopIssueAttachmentUpload{
		FileName:  name,
		SizeBytes: size,
		Open: func() (service.DesktopIssueAttachmentFile, error) {
			return os.Open(f.Name())
		},
	}
}

// Exercise the full large-file read and hash without writing another copy to disk.
type feedbackDiscardStore struct {
	service.DesktopIssueAttachmentStore
}

func (feedbackDiscardStore) Upload(_ context.Context, _ string, body io.Reader, _ string) (int64, error) {
	return io.Copy(io.Discard, body)
}

func TestDesktopIssueUploadAndReadPrivateFiles(t *testing.T) {
	dataDir, spoolDir := feedbackTestDirs(t)
	repo := &feedbackIssueRepo{}
	svc := service.NewDesktopIssueService(repo, nil, repository.ProvideDesktopIssueAttachmentStore())
	png := []byte("\x89PNG\r\n\x1a\n")
	jpeg := []byte("\xff\xd8\xff\xe0")
	webp := []byte("RIFF\x04\x00\x00\x00WEBPVP8 ")
	gif := []byte("GIF89a")
	mp4 := []byte("\x00\x00\x00\x14ftypisom\x00\x00\x00\x00mp42")
	webm := []byte("\x1a\x45\xdf\xa3")
	mov := []byte("\x00\x00\x00\x14ftypqt  \x00\x00\x00\x00qt  ")
	archive := feedbackDiagnosticsZIP(t)
	for _, tc := range []struct {
		name, declared, wantMIME, wantName string
		body                               []byte
	}{
		{"screen.PNG", "image/svg+xml", "image/png", "screen.PNG", png},
		{"photo.exe", "IMAGE/JPG; foo=bar", "image/jpeg", "photo.jpg", jpeg},
		{"picture.webp", "application/octet-stream", "image/webp", "picture.webp", webp},
		{"steps.gif", "image/gif", "image/gif", "steps.gif", gif},
		{"capture.mp4", "video/mp4", "video/mp4", "capture.mp4", mp4},
		{"capture.webm", "video/webm", "video/webm", "capture.webm", webm},
		{"capture.mov", "application/octet-stream", "video/quicktime", "capture.mov", mov},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := submitFeedbackFiles(t, svc, []feedbackPart{
				{field: "attachments", name: tc.name, mime: tc.declared, body: tc.body},
				{field: "diagnostics", name: "report.txt", mime: "text/plain", body: archive},
			})
			require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
			require.Len(t, repo.detail.Attachments, 2)
			attachment := repo.detail.Attachments[0]
			require.Equal(t, tc.wantMIME, attachment.MimeType)
			require.Equal(t, tc.wantName, attachment.FileName)
			require.EqualValues(t, len(tc.body), attachment.SizeBytes)
			digest := sha256.Sum256(tc.body)
			require.Equal(t, hex.EncodeToString(digest[:]), attachment.SHA256)
			stored, err := os.ReadFile(filepath.Join(dataDir, "desktop-issue-attachments", filepath.FromSlash(attachment.ObjectKey)))
			require.NoError(t, err)
			require.Equal(t, tc.body, stored)

			router := gin.New()
			h := admin.NewDesktopIssueHandler(svc)
			router.GET("/issues/:id/attachments/:attachmentId", h.DownloadAttachment)
			preview := httptest.NewRecorder()
			router.ServeHTTP(preview, httptest.NewRequest(http.MethodGet, "/issues/1/attachments/1", nil))
			require.Equal(t, http.StatusOK, preview.Code, preview.Body.String())
			require.Equal(t, tc.wantMIME, preview.Header().Get("Content-Type"))
			require.Equal(t, "nosniff", preview.Header().Get("X-Content-Type-Options"))
			require.Equal(t, tc.body, preview.Body.Bytes())

			require.Equal(t, "report.zip", repo.detail.Attachments[1].FileName)
			entries, err := svc.InspectDiagnosticsManifest(t.Context(), 1, 2)
			require.NoError(t, err)
			require.Equal(t, []model.DesktopIssueManifestEntry{{Name: "a.txt", SizeBytes: 1}, {Name: "z.txt", SizeBytes: 1}}, entries)
			spooled, err := os.ReadDir(spoolDir)
			require.NoError(t, err)
			require.Empty(t, spooled)
		})
	}
}

func TestDesktopIssueUploadCleansFilesOnFailure(t *testing.T) {
	for _, tc := range []struct {
		name, reason string
		status       int
		parts        []feedbackPart
		partialWrite bool
		createErr    error
	}{
		{
			name: "disguised_media", status: http.StatusBadRequest, reason: "UNSUPPORTED_ATTACHMENT_TYPE",
			parts: []feedbackPart{{"attachments", "screen.png", "image/png", []byte("<html>not an image</html>")}},
		},
		{
			name: "invalid_zip", status: http.StatusBadRequest, reason: "INVALID_DIAGNOSTICS_ARCHIVE",
			parts: []feedbackPart{{"diagnostics", "broken.zip", "application/zip", []byte("not a zip file")}},
		},
		{
			name: "duplicate_diagnostics", status: http.StatusBadRequest, reason: "TOO_MANY_DIAGNOSTICS",
			parts: []feedbackPart{
				{"diagnostics", "one.zip", "application/zip", []byte("one")},
				{"diagnostics", "two.zip", "application/zip", []byte("two")},
			},
		},
		{
			name: "partial_write", status: http.StatusInternalServerError, reason: "ISSUE_ATTACHMENT_UPLOAD_FAILED",
			parts: []feedbackPart{{"attachments", "second.png", "image/png", []byte("\x89PNG\r\n\x1a\n")}}, partialWrite: true,
		},
		{
			name: "create_failed", status: http.StatusInternalServerError,
			createErr: errors.New("create failed"),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dataDir, spoolDir := feedbackTestDirs(t)
			store := repository.ProvideDesktopIssueAttachmentStore()
			if tc.partialWrite {
				store = &feedbackFailingStore{DesktopIssueAttachmentStore: store}
			}
			repo := &feedbackIssueRepo{createErr: tc.createErr}
			svc := service.NewDesktopIssueService(repo, nil, store)
			parts := []feedbackPart{{"attachments", "first.png", "image/png", []byte("\x89PNG\r\n\x1a\n")}}
			w := submitFeedbackFiles(t, svc, append(parts, tc.parts...))
			require.Equal(t, tc.status, w.Code, w.Body.String())
			if tc.reason != "" {
				require.Contains(t, w.Body.String(), tc.reason)
			}
			require.Nil(t, repo.detail)
			for _, dir := range []string{dataDir, spoolDir} {
				require.NoError(t, filepath.WalkDir(dir, func(p string, entry os.DirEntry, err error) error {
					require.NoError(t, err)
					require.True(t, entry.IsDir(), "uncommitted file left at %s", p)
					return nil
				}))
			}
		})
	}
}

type feedbackIssueRepo struct {
	service.DesktopIssueRepository
	detail    *model.DesktopIssueDetail
	createErr error
}

func (r *feedbackIssueRepo) Create(_ context.Context, issue *model.DesktopIssue, attachments []*model.DesktopIssueAttachment, event *model.DesktopIssueEvent) (*model.DesktopIssueDetail, error) {
	if r.createErr != nil {
		return nil, r.createErr
	}
	issue.ID = 1
	for i, a := range attachments {
		a.ID = int64(i + 1)
		a.IssueID = issue.ID
	}
	r.detail = &model.DesktopIssueDetail{Issue: issue, Attachments: attachments, Events: []*model.DesktopIssueEvent{event}}
	return r.detail, nil
}

func (r *feedbackIssueRepo) GetAttachment(_ context.Context, issueID, attachmentID int64) (*model.DesktopIssueAttachment, error) {
	for _, a := range r.detail.Attachments {
		if a.ID == attachmentID && a.IssueID == issueID {
			return a, nil
		}
	}
	return nil, service.ErrDesktopIssueAttachmentNotFound
}

type feedbackFailingStore struct {
	service.DesktopIssueAttachmentStore
	firstStored bool
}

func (s *feedbackFailingStore) Upload(ctx context.Context, key string, body io.Reader, contentType string) (int64, error) {
	if !s.firstStored {
		s.firstStored = true
		return s.DesktopIssueAttachmentStore.Upload(ctx, key, body, contentType)
	}
	return s.DesktopIssueAttachmentStore.Upload(ctx, key, io.MultiReader(io.LimitReader(body, 4), feedbackFailedReader{}), contentType)
}

type feedbackFailedReader struct{}

func (feedbackFailedReader) Read([]byte) (int, error) { return 0, errors.New("upload interrupted") }

type feedbackZeroReader struct{}

func (feedbackZeroReader) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}

type feedbackPart struct {
	field, name, mime string
	body              []byte
}

func submitFeedbackFiles(t *testing.T, svc *service.DesktopIssueService, parts []feedbackPart) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	require.NoError(t, form.WriteField("title", "Feedback"))
	for _, part := range parts {
		w, err := form.CreatePart(textproto.MIMEHeader{
			"Content-Disposition": {multipart.FileContentDisposition(part.field, part.name)},
			"Content-Type":        {part.mime},
		})
		require.NoError(t, err)
		_, err = w.Write(part.body)
		require.NoError(t, err)
	}
	require.NoError(t, form.Close())
	req := httptest.NewRequest(http.MethodPost, feedbackSubmitPath, &body)
	req.Header.Set("Content-Type", form.FormDataContentType())
	w := httptest.NewRecorder()
	feedbackSubmitRouter(svc).ServeHTTP(w, req)
	return w
}

func feedbackTestDirs(t *testing.T) (dataDir, spoolDir string) {
	t.Helper()
	dataDir, spoolDir = t.TempDir(), t.TempDir()
	t.Setenv("DATA_DIR", dataDir)
	for _, key := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(key, spoolDir)
	}
	return dataDir, spoolDir
}

func feedbackDiagnosticsZIP(t *testing.T) []byte {
	t.Helper()
	var body bytes.Buffer
	w := zip.NewWriter(&body)
	_, err := w.Create("directory/")
	require.NoError(t, err)
	for _, name := range []string{"z.txt", "a.txt"} {
		entry, err := w.Create(name)
		require.NoError(t, err)
		_, err = io.WriteString(entry, "x")
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())
	return body.Bytes()
}
