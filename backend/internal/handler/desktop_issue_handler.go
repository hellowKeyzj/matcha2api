package handler

import (
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const desktopIssueSubmitMaxBodyBytes = 256 << 20

var errDesktopIssueUnavailable = infraerrors.ServiceUnavailable("DESKTOP_ISSUE_UNAVAILABLE", "feedback service unavailable")

type DesktopIssueHandler struct {
	issueService *service.DesktopIssueService
}

func NewDesktopIssueHandler(issueService *service.DesktopIssueService) *DesktopIssueHandler {
	return &DesktopIssueHandler{issueService: issueService}
}

type desktopIssueSubmitResponse struct {
	ID              int64  `json:"id"`
	Title           string `json:"title"`
	Status          string `json:"status"`
	Kind            string `json:"kind"`
	Severity        string `json:"severity"`
	AttachmentCount int    `json:"attachment_count"`
	HasDiagnostics  bool   `json:"has_diagnostics"`
	CreatedAt       string `json:"created_at"`
}

// Submit handles POST /api/v1/feedback/issues (authenticated submit-only endpoint).
func (h *DesktopIssueHandler) Submit(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if h == nil || h.issueService == nil {
		response.ErrorFrom(c, errDesktopIssueUnavailable)
		return
	}

	err := parseDesktopIssueSubmitForm(c)
	form := c.Request.MultipartForm
	if form != nil {
		defer func() { _ = form.RemoveAll() }()
	}
	if err != nil {
		if errors.Is(err, multipart.ErrMessageTooLarge) {
			response.Error(c, http.StatusRequestEntityTooLarge, "feedback multipart payload exceeds form limits")
			return
		}
		failure := pkghttputil.DescribeRequestBodyReadFailure(c.Request, err)
		if failure.Status == http.StatusBadRequest {
			response.BadRequest(c, "invalid feedback multipart payload")
		} else {
			response.Error(c, failure.Status, failure.ClientMessage())
		}
		return
	}

	headers := form.File["attachments"]
	media := make([]service.DesktopIssueAttachmentUpload, 0, len(headers))
	for _, fh := range headers {
		media = append(media, desktopIssueAttachmentUpload(fh))
	}

	var diagnostics *service.DesktopIssueAttachmentUpload
	diagFiles := form.File["diagnostics"]
	switch len(diagFiles) {
	case 0:
	case 1:
		upload := desktopIssueAttachmentUpload(diagFiles[0])
		diagnostics = &upload
	default:
		response.ErrorFrom(c, infraerrors.BadRequest("TOO_MANY_DIAGNOSTICS", "at most one diagnostics archive is allowed"))
		return
	}

	fields := c.Request.PostForm
	var clientMeta map[string]any
	if rawMeta := strings.TrimSpace(fields.Get("client_meta")); rawMeta != "" {
		_ = json.Unmarshal([]byte(rawMeta), &clientMeta)
	}

	detail, err := h.issueService.Submit(c.Request.Context(), service.SubmitDesktopIssueRequest{
		ReporterUserID: subject.UserID,
		Title:          fields.Get("title"),
		Description:    fields.Get("description"),
		Kind:           fields.Get("kind"),
		Severity:       fields.Get("severity"),
		AppVersion:     fields.Get("app_version"),
		ClientOS:       fields.Get("client_os"),
		ClientArch:     fields.Get("client_arch"),
		ClientRoute:    fields.Get("client_route"),
		ClientLocale:   fields.Get("client_locale"),
		ClientMeta:     clientMeta,
		Media:          media,
		Diagnostics:    diagnostics,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Created(c, desktopIssueSubmitResponse{
		ID:              detail.Issue.ID,
		Title:           detail.Issue.Title,
		Status:          detail.Issue.Status,
		Kind:            detail.Issue.Kind,
		Severity:        detail.Issue.Severity,
		AttachmentCount: detail.Issue.ImageCount + detail.Issue.VideoCount,
		HasDiagnostics:  detail.Issue.HasDiagnostics,
		CreatedAt:       detail.Issue.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	})
}

func parseDesktopIssueSubmitForm(c *gin.Context) error {
	req := c.Request
	if req.ContentLength > desktopIssueSubmitMaxBodyBytes {
		return &http.MaxBytesError{Limit: desktopIssueSubmitMaxBodyBytes}
	}
	req.Body = http.MaxBytesReader(c.Writer, req.Body, desktopIssueSubmitMaxBodyBytes)
	if err := req.ParseMultipartForm(0); err != nil {
		return err
	}
	// The multipart parser stops at the final boundary. Count any epilogue against the same limit.
	_, err := io.Copy(io.Discard, req.Body)
	return err
}

func desktopIssueAttachmentUpload(fh *multipart.FileHeader) service.DesktopIssueAttachmentUpload {
	return service.DesktopIssueAttachmentUpload{
		FileName:  fh.Filename,
		MimeType:  fh.Header.Get("Content-Type"),
		SizeBytes: fh.Size,
		Open: func() (service.DesktopIssueAttachmentFile, error) {
			return fh.Open()
		},
	}
}
