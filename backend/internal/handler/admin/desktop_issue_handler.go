package admin

import (
	"mime"
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/model"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

var errAdminDesktopIssueUnavailable = infraerrors.ServiceUnavailable("DESKTOP_ISSUE_UNAVAILABLE", "feedback service unavailable")

type DesktopIssueHandler struct {
	issueService *service.DesktopIssueService
}

func NewDesktopIssueHandler(issueService *service.DesktopIssueService) *DesktopIssueHandler {
	return &DesktopIssueHandler{issueService: issueService}
}

type adminDesktopIssueListResponse struct {
	Items    []*model.DesktopIssue          `json:"items"`
	Total    int64                          `json:"total"`
	Page     int                            `json:"page"`
	PageSize int                            `json:"page_size"`
	Pages    int                            `json:"pages"`
	Counts   model.DesktopIssueStatusCounts `json:"counts"`
}

type adminDesktopIssueUpdateRequest struct {
	Status       *string   `json:"status"`
	Severity     *string   `json:"severity"`
	Labels       *[]string `json:"labels"`
	AssigneeName *string   `json:"assignee_name"`
	FixVersion   *string   `json:"fix_version"`
}

type adminDesktopIssueNoteRequest struct {
	Body       string  `json:"body"`
	Status     *string `json:"status"`
	FixVersion *string `json:"fix_version"`
}

type adminDesktopIssueBatchStatusRequest struct {
	IDs    []int64 `json:"ids"`
	Status string  `json:"status"`
}

func (h *DesktopIssueHandler) List(c *gin.Context) {
	if h == nil || h.issueService == nil {
		response.ErrorFrom(c, errAdminDesktopIssueUnavailable)
		return
	}
	page, pageSize := response.ParsePagination(c)
	pageSize = min(pageSize, 100)
	var hasDiag *bool
	if raw := strings.TrimSpace(c.Query("has_diagnostics")); raw != "" {
		if v, err := strconv.ParseBool(raw); err == nil {
			hasDiag = &v
		}
	}
	result, err := h.issueService.List(c.Request.Context(), service.DesktopIssueListFilter{
		Status:         strings.TrimSpace(c.Query("status")),
		Kind:           strings.TrimSpace(c.Query("kind")),
		Severity:       strings.TrimSpace(c.Query("severity")),
		HasDiagnostics: hasDiag,
		Reporter:       strings.TrimSpace(c.Query("reporter")),
		Search:         strings.TrimSpace(c.Query("search")),
		Page:           page,
		PageSize:       pageSize,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	pages := int(result.Total) / pageSize
	if int(result.Total)%pageSize > 0 {
		pages++
	}
	response.Success(c, adminDesktopIssueListResponse{
		Items:    result.Items,
		Total:    result.Total,
		Page:     page,
		PageSize: pageSize,
		Pages:    pages,
		Counts:   result.Counts,
	})
}

func (h *DesktopIssueHandler) GetByID(c *gin.Context) {
	if h == nil || h.issueService == nil {
		response.ErrorFrom(c, errAdminDesktopIssueUnavailable)
		return
	}
	id, ok := parsePositiveInt64Param(c, "id")
	if !ok {
		return
	}
	detail, err := h.issueService.GetDetail(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, detail)
}

func (h *DesktopIssueHandler) Update(c *gin.Context) {
	if h == nil || h.issueService == nil {
		response.ErrorFrom(c, errAdminDesktopIssueUnavailable)
		return
	}
	id, ok := parsePositiveInt64Param(c, "id")
	if !ok {
		return
	}
	var req adminDesktopIssueUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid update payload")
		return
	}
	actorID := adminActorUserID(c)
	detail, err := h.issueService.Update(c.Request.Context(), service.UpdateDesktopIssueRequest{
		IssueID:      id,
		ActorUserID:  actorID,
		Status:       req.Status,
		Severity:     req.Severity,
		Labels:       req.Labels,
		AssigneeName: req.AssigneeName,
		FixVersion:   req.FixVersion,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, detail)
}

func (h *DesktopIssueHandler) AddNote(c *gin.Context) {
	if h == nil || h.issueService == nil {
		response.ErrorFrom(c, errAdminDesktopIssueUnavailable)
		return
	}
	id, ok := parsePositiveInt64Param(c, "id")
	if !ok {
		return
	}
	var req adminDesktopIssueNoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid note payload")
		return
	}
	actorID := adminActorUserID(c)
	detail, err := h.issueService.AddNote(c.Request.Context(), service.AddDesktopIssueNoteRequest{
		IssueID:     id,
		ActorUserID: actorID,
		Body:        req.Body,
		Status:      req.Status,
		FixVersion:  req.FixVersion,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, detail)
}

func (h *DesktopIssueHandler) BatchUpdateStatus(c *gin.Context) {
	if h == nil || h.issueService == nil {
		response.ErrorFrom(c, errAdminDesktopIssueUnavailable)
		return
	}
	var req adminDesktopIssueBatchStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid batch update payload")
		return
	}
	updated, err := h.issueService.BatchUpdateStatus(c.Request.Context(), req.IDs, req.Status, adminActorUserID(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"updated": updated})
}

func (h *DesktopIssueHandler) DownloadAttachment(c *gin.Context) {
	if h == nil || h.issueService == nil {
		response.ErrorFrom(c, errAdminDesktopIssueUnavailable)
		return
	}
	issueID, ok := parsePositiveInt64Param(c, "id")
	if !ok {
		return
	}
	attachmentID, ok := parsePositiveInt64Param(c, "attachmentId")
	if !ok {
		return
	}
	stream, err := h.issueService.OpenAttachment(c.Request.Context(), issueID, attachmentID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	defer func() { _ = stream.Reader.Close() }()

	disposition := "inline"
	if c.Query("download") == "1" || stream.Attachment.Category == model.DesktopIssueAttachmentDiagnostics {
		disposition = "attachment"
	}
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Cache-Control", "private, max-age=300")
	c.Header("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": stream.Attachment.FileName}))
	contentType := stream.Attachment.MimeType
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.DataFromReader(http.StatusOK, stream.Attachment.SizeBytes, contentType, stream.Reader, nil)
}

func (h *DesktopIssueHandler) GetDiagnosticsManifest(c *gin.Context) {
	if h == nil || h.issueService == nil {
		response.ErrorFrom(c, errAdminDesktopIssueUnavailable)
		return
	}
	issueID, ok := parsePositiveInt64Param(c, "id")
	if !ok {
		return
	}
	attachmentID, ok := parsePositiveInt64Param(c, "attachmentId")
	if !ok {
		return
	}
	entries, err := h.issueService.InspectDiagnosticsManifest(c.Request.Context(), issueID, attachmentID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"entries": entries})
}

func parsePositiveInt64Param(c *gin.Context, name string) (int64, bool) {
	v, err := strconv.ParseInt(strings.TrimSpace(c.Param(name)), 10, 64)
	if err != nil || v <= 0 {
		response.BadRequest(c, "invalid "+name)
		return 0, false
	}
	return v, true
}

func adminActorUserID(c *gin.Context) int64 {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		return 0
	}
	return subject.UserID
}
