package service

import (
	"archive/zip"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/model"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	DesktopIssueMaxTitleRunes       = 200
	DesktopIssueMaxDescriptionRunes = 20000
	DesktopIssueMaxMediaAttachments = 6
	DesktopIssueMaxImageBytes       = 10 << 20 // 10 MiB
	DesktopIssueMaxVideoBytes       = 50 << 20 // 50 MiB
	DesktopIssueMaxDiagnosticsBytes = 50 << 20 // 50 MiB
	DesktopIssueMaxAttachmentBytes  = 250 << 20
	DesktopIssueMaxLabels           = 12
	DesktopIssueMaxLabelRunes       = 32
	DesktopIssueMaxNoteRunes        = 10000
)

var (
	ErrDesktopIssueNotFound           = infraerrors.NotFound("DESKTOP_ISSUE_NOT_FOUND", "issue not found")
	ErrDesktopIssueAttachmentNotFound = infraerrors.NotFound("DESKTOP_ISSUE_ATTACHMENT_NOT_FOUND", "issue attachment not found")
)

type DesktopIssueListFilter struct {
	Status         string
	Kind           string
	Severity       string
	HasDiagnostics *bool
	Reporter       string
	Search         string
	Page           int
	PageSize       int
}

type DesktopIssueListResult struct {
	Items  []*model.DesktopIssue
	Total  int64
	Counts model.DesktopIssueStatusCounts
}

type DesktopIssueRepository interface {
	Create(ctx context.Context, issue *model.DesktopIssue, attachments []*model.DesktopIssueAttachment, initialEvent *model.DesktopIssueEvent) (*model.DesktopIssueDetail, error)
	List(ctx context.Context, filter DesktopIssueListFilter) (*DesktopIssueListResult, error)
	GetDetail(ctx context.Context, id int64) (*model.DesktopIssueDetail, error)
	GetAttachment(ctx context.Context, issueID, attachmentID int64) (*model.DesktopIssueAttachment, error)
	// Mutate applies a pure change to each current locked row; no events means no write.
	Mutate(ctx context.Context, ids []int64, apply func(*model.DesktopIssue, time.Time) []*model.DesktopIssueEvent) (int64, error)
}

type DesktopIssueAttachmentStore interface {
	Upload(ctx context.Context, key string, body io.Reader, contentType string) (int64, error)
	Download(ctx context.Context, key string) (DesktopIssueAttachmentFile, error)
	Delete(ctx context.Context, key string) error
}

type DesktopIssueAttachmentFile interface {
	io.ReadCloser
	io.ReaderAt
}

type DesktopIssueAttachmentUpload struct {
	FileName  string
	MimeType  string
	SizeBytes int64
	Open      func() (DesktopIssueAttachmentFile, error)
}

type SubmitDesktopIssueRequest struct {
	ReporterUserID int64
	Title          string
	Description    string
	Kind           string
	Severity       string
	AppVersion     string
	ClientOS       string
	ClientArch     string
	ClientRoute    string
	ClientLocale   string
	ClientMeta     map[string]any
	Media          []DesktopIssueAttachmentUpload
	Diagnostics    *DesktopIssueAttachmentUpload
}

type UpdateDesktopIssueRequest struct {
	IssueID      int64
	ActorUserID  int64
	Status       *string
	Severity     *string
	Labels       *[]string
	AssigneeName *string
	FixVersion   *string
}

type AddDesktopIssueNoteRequest struct {
	IssueID     int64
	ActorUserID int64
	Body        string
	Status      *string
	FixVersion  *string
}

type DesktopIssueAttachmentStream struct {
	Attachment *model.DesktopIssueAttachment
	Reader     DesktopIssueAttachmentFile
}

type DesktopIssueService struct {
	repo     DesktopIssueRepository
	userRepo UserRepository
	store    DesktopIssueAttachmentStore
}

func NewDesktopIssueService(
	repo DesktopIssueRepository,
	userRepo UserRepository,
	store DesktopIssueAttachmentStore,
) *DesktopIssueService {
	return &DesktopIssueService{
		repo:     repo,
		userRepo: userRepo,
		store:    store,
	}
}

func (s *DesktopIssueService) Submit(ctx context.Context, req SubmitDesktopIssueRequest) (*model.DesktopIssueDetail, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("DESKTOP_ISSUE_UNAVAILABLE", "feedback service unavailable")
	}
	if req.ReporterUserID <= 0 {
		return nil, infraerrors.Unauthorized("UNAUTHORIZED", "authentication required")
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		return nil, infraerrors.BadRequest("INVALID_ISSUE_TITLE", "title is required")
	}
	if len([]rune(title)) > DesktopIssueMaxTitleRunes {
		return nil, infraerrors.BadRequest("INVALID_ISSUE_TITLE", "title is too long")
	}
	description := strings.TrimSpace(req.Description)
	if len([]rune(description)) > DesktopIssueMaxDescriptionRunes {
		return nil, infraerrors.BadRequest("INVALID_ISSUE_DESCRIPTION", "description is too long")
	}

	kind := normalizeDesktopIssueKind(req.Kind)
	if kind == "" {
		return nil, infraerrors.BadRequest("INVALID_ISSUE_KIND", "invalid issue kind")
	}
	severity := normalizeDesktopIssueSeverity(req.Severity)
	if severity == "" {
		if kind == model.DesktopIssueKindFeature {
			severity = model.DesktopIssueSeverityMinor
		} else {
			severity = model.DesktopIssueSeverityMajor
		}
	}

	if len(req.Media) > DesktopIssueMaxMediaAttachments {
		return nil, infraerrors.BadRequest("TOO_MANY_ATTACHMENTS", fmt.Sprintf("at most %d media attachments are allowed", DesktopIssueMaxMediaAttachments))
	}

	var attachmentBytes int64
	for _, media := range req.Media {
		attachmentBytes += media.SizeBytes
	}
	hasDiagnostics := req.Diagnostics != nil
	if hasDiagnostics {
		attachmentBytes += req.Diagnostics.SizeBytes
	}
	if attachmentBytes > DesktopIssueMaxAttachmentBytes {
		return nil, infraerrors.Newf(http.StatusRequestEntityTooLarge, "ATTACHMENTS_TOO_LARGE", "attachments must total %d MiB or less", DesktopIssueMaxAttachmentBytes>>20)
	}
	if (len(req.Media) > 0 || hasDiagnostics) && s.store == nil {
		return nil, infraerrors.ServiceUnavailable("ISSUE_STORAGE_UNAVAILABLE", "attachment storage is not configured")
	}

	reporterEmail, reporterUsername := s.resolveActorIdentity(ctx, req.ReporterUserID)
	reporterID := req.ReporterUserID

	attachments := make([]*model.DesktopIssueAttachment, 0, len(req.Media)+1)
	committed := false
	defer func() {
		if committed {
			return
		}
		for _, attachment := range attachments {
			s.removeStoredAttachment(ctx, attachment.ObjectKey)
		}
	}()

	now := time.Now().UTC()
	imageCount := 0
	videoCount := 0
	for _, upload := range req.Media {
		attachment, err := s.storeAttachment(ctx, now, upload, false)
		if err != nil {
			return nil, err
		}
		if attachment.Category == model.DesktopIssueAttachmentImage {
			imageCount++
		} else {
			videoCount++
		}
		attachments = append(attachments, attachment)
	}
	if hasDiagnostics {
		attachment, err := s.storeAttachment(ctx, now, *req.Diagnostics, true)
		if err != nil {
			return nil, err
		}
		attachments = append(attachments, attachment)
	}

	defaultLabels := []string{kind}
	if hasDiagnostics {
		defaultLabels = append(defaultLabels, "diagnostics")
	}

	clientMeta := req.ClientMeta
	if clientMeta == nil {
		clientMeta = map[string]any{}
	}

	issue := &model.DesktopIssue{
		ReporterUserID:   &reporterID,
		ReporterEmail:    reporterEmail,
		ReporterUsername: reporterUsername,
		Title:            title,
		Description:      description,
		Kind:             kind,
		Severity:         severity,
		Status:           model.DesktopIssueStatusOpen,
		Labels:           defaultLabels,
		AppVersion:       truncateRunes(strings.TrimSpace(req.AppVersion), 64),
		ClientOS:         truncateRunes(strings.TrimSpace(req.ClientOS), 128),
		ClientArch:       truncateRunes(strings.TrimSpace(req.ClientArch), 32),
		ClientRoute:      truncateRunes(strings.TrimSpace(req.ClientRoute), 255),
		ClientLocale:     truncateRunes(strings.TrimSpace(req.ClientLocale), 32),
		ClientMeta:       clientMeta,
		HasDiagnostics:   hasDiagnostics,
		ImageCount:       imageCount,
		VideoCount:       videoCount,
	}

	actorLabel := displayIdentity(reporterEmail, reporterUsername, req.ReporterUserID)
	initialEvent := &model.DesktopIssueEvent{
		EventType:   model.DesktopIssueEventCreated,
		ActorUserID: &reporterID,
		ActorName:   actorLabel,
		Metadata: map[string]any{
			"app_version":     issue.AppVersion,
			"client_os":       issue.ClientOS,
			"has_diagnostics": hasDiagnostics,
			"media_count":     imageCount + videoCount,
		},
	}

	detail, err := s.repo.Create(ctx, issue, attachments, initialEvent)
	if err != nil {
		return nil, err
	}
	committed = true
	return detail, nil
}

func (s *DesktopIssueService) List(ctx context.Context, filter DesktopIssueListFilter) (*DesktopIssueListResult, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("DESKTOP_ISSUE_UNAVAILABLE", "feedback service unavailable")
	}
	return s.repo.List(ctx, filter)
}

func (s *DesktopIssueService) GetDetail(ctx context.Context, id int64) (*model.DesktopIssueDetail, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("DESKTOP_ISSUE_UNAVAILABLE", "feedback service unavailable")
	}
	if id <= 0 {
		return nil, ErrDesktopIssueNotFound
	}
	return s.repo.GetDetail(ctx, id)
}

func (s *DesktopIssueService) OpenAttachment(ctx context.Context, issueID, attachmentID int64) (*DesktopIssueAttachmentStream, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("DESKTOP_ISSUE_UNAVAILABLE", "feedback service unavailable")
	}
	att, err := s.repo.GetAttachment(ctx, issueID, attachmentID)
	if err != nil {
		return nil, err
	}
	if s.store == nil {
		return nil, infraerrors.ServiceUnavailable("ISSUE_STORAGE_UNAVAILABLE", "attachment storage is not configured")
	}
	reader, err := s.store.Download(ctx, att.ObjectKey)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, infraerrors.NotFound("ISSUE_ATTACHMENT_OBJECT_MISSING", "attachment object could not be read").WithCause(err)
		}
		return nil, infraerrors.InternalServer("ISSUE_ATTACHMENT_READ_FAILED", "failed to read attachment").WithCause(err)
	}
	return &DesktopIssueAttachmentStream{
		Attachment: att,
		Reader:     reader,
	}, nil
}

func (s *DesktopIssueService) InspectDiagnosticsManifest(ctx context.Context, issueID, attachmentID int64) ([]model.DesktopIssueManifestEntry, error) {
	stream, err := s.OpenAttachment(ctx, issueID, attachmentID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = stream.Reader.Close() }()
	if stream.Attachment.Category != model.DesktopIssueAttachmentDiagnostics {
		return nil, infraerrors.BadRequest("NOT_DIAGNOSTICS_ARCHIVE", "attachment is not a diagnostics archive")
	}
	if stream.Attachment.SizeBytes > DesktopIssueMaxDiagnosticsBytes {
		return nil, infraerrors.BadRequest("DIAGNOSTICS_TOO_LARGE", "diagnostics archive exceeds size limit")
	}
	zr, err := zip.NewReader(stream.Reader, stream.Attachment.SizeBytes)
	if err != nil {
		return nil, infraerrors.BadRequest("INVALID_DIAGNOSTICS_ARCHIVE", "diagnostics archive is not a valid zip file")
	}
	entries := make([]model.DesktopIssueManifestEntry, 0, len(zr.File))
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		entries = append(entries, model.DesktopIssueManifestEntry{
			Name:      f.Name,
			SizeBytes: f.UncompressedSize64,
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries, nil
}

func (s *DesktopIssueService) resolveActorIdentity(ctx context.Context, userID int64) (email string, username string) {
	if s.userRepo == nil || userID <= 0 {
		return "", ""
	}
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil || user == nil {
		return "", ""
	}
	return strings.TrimSpace(user.Email), strings.TrimSpace(user.Username)
}

func displayIdentity(email, username string, userID int64) string {
	if email != "" {
		return email
	}
	if username != "" {
		return username
	}
	return fmt.Sprintf("user#%d", userID)
}

func normalizeDesktopIssueKind(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", model.DesktopIssueKindBug:
		return model.DesktopIssueKindBug
	case model.DesktopIssueKindFeature:
		return model.DesktopIssueKindFeature
	case model.DesktopIssueKindExperience:
		return model.DesktopIssueKindExperience
	default:
		return ""
	}
}

func normalizeDesktopIssueSeverity(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case model.DesktopIssueSeverityBlocker:
		return model.DesktopIssueSeverityBlocker
	case model.DesktopIssueSeverityMajor:
		return model.DesktopIssueSeverityMajor
	case model.DesktopIssueSeverityMinor:
		return model.DesktopIssueSeverityMinor
	default:
		return ""
	}
}

func (s *DesktopIssueService) storeAttachment(ctx context.Context, now time.Time, upload DesktopIssueAttachmentUpload, diagnostics bool) (*model.DesktopIssueAttachment, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if upload.SizeBytes <= 0 {
		if diagnostics {
			return nil, infraerrors.BadRequest("EMPTY_DIAGNOSTICS", "diagnostics archive is empty")
		}
		return nil, infraerrors.BadRequest("EMPTY_ATTACHMENT", "attachment file is empty")
	}
	if diagnostics && upload.SizeBytes > DesktopIssueMaxDiagnosticsBytes {
		return nil, infraerrors.Newf(http.StatusRequestEntityTooLarge, "DIAGNOSTICS_TOO_LARGE", "diagnostics archive must be %d MiB or smaller", DesktopIssueMaxDiagnosticsBytes>>20)
	}

	file, err := upload.Open()
	if err != nil {
		return nil, infraerrors.InternalServer("ISSUE_ATTACHMENT_READ_FAILED", "failed to read attachment").WithCause(err)
	}
	defer func() { _ = file.Close() }()

	attachment := &model.DesktopIssueAttachment{SizeBytes: upload.SizeBytes}
	if diagnostics {
		if _, err := zip.NewReader(file, upload.SizeBytes); err != nil {
			return nil, infraerrors.BadRequest("INVALID_DIAGNOSTICS_ARCHIVE", "diagnostics archive must be a valid zip file")
		}
		attachment.Category = model.DesktopIssueAttachmentDiagnostics
		attachment.MimeType = "application/zip"
		attachment.FileName = desktopIssueAttachmentFileName(upload.FileName, "diagnostics", ".zip")
	} else if err := describeDesktopIssueMedia(attachment, upload, file); err != nil {
		return nil, err
	}

	attachment.ObjectKey = path.Join(now.Format("2006/01/02"), attachment.Category, rand.Text())
	hasher := sha256.New()
	body := io.TeeReader(io.NewSectionReader(file, 0, upload.SizeBytes), hasher)
	written, err := s.store.Upload(ctx, attachment.ObjectKey, body, attachment.MimeType)
	if err == nil && written != upload.SizeBytes {
		err = io.ErrUnexpectedEOF
	}
	if err != nil {
		s.removeStoredAttachment(ctx, attachment.ObjectKey)
		return nil, infraerrors.InternalServer("ISSUE_ATTACHMENT_UPLOAD_FAILED", "failed to store attachment").WithCause(err)
	}
	attachment.SHA256 = hex.EncodeToString(hasher.Sum(nil))
	return attachment, nil
}

func (s *DesktopIssueService) removeStoredAttachment(ctx context.Context, key string) {
	ctx = context.WithoutCancel(ctx)
	if err := s.store.Delete(ctx, key); err != nil && !errors.Is(err, fs.ErrNotExist) {
		slog.WarnContext(ctx, "failed to remove uncommitted desktop issue attachment", "key", key, "error", err)
	}
}

var desktopIssueMediaExtensions = map[string]string{
	"image/png":       ".png",
	"image/jpeg":      ".jpg",
	"image/webp":      ".webp",
	"image/gif":       ".gif",
	"video/mp4":       ".mp4",
	"video/webm":      ".webm",
	"video/quicktime": ".mov",
}

func describeDesktopIssueMedia(attachment *model.DesktopIssueAttachment, upload DesktopIssueAttachmentUpload, file io.ReaderAt) error {
	var header [512]byte
	n, err := file.ReadAt(header[:], 0)
	if err != nil && err != io.EOF {
		return infraerrors.InternalServer("ISSUE_ATTACHMENT_READ_FAILED", "failed to read attachment").WithCause(err)
	}
	declared, _, _ := strings.Cut(upload.MimeType, ";")
	declared = strings.ToLower(strings.TrimSpace(declared))
	if declared == "image/jpg" {
		declared = "image/jpeg"
	}
	fileExt := strings.ToLower(path.Ext(upload.FileName))
	mimeType := detectDesktopIssueMediaMIME(header[:n], upload.SizeBytes, fileExt == ".mov" || declared == "video/quicktime")
	ext, allowed := desktopIssueMediaExtensions[mimeType]
	genericMIME := declared == "" || declared == "application/octet-stream"
	if !allowed || (!genericMIME && declared != mimeType && !desktopIssueExtensionMatches(fileExt, ext)) {
		return infraerrors.BadRequest("UNSUPPORTED_ATTACHMENT_TYPE", "only PNG, JPEG, WebP, GIF images and MP4, WebM, MOV videos are allowed")
	}
	category, limit, reason := model.DesktopIssueAttachmentImage, DesktopIssueMaxImageBytes, "IMAGE_TOO_LARGE"
	if strings.HasPrefix(mimeType, "video/") {
		category, limit, reason = model.DesktopIssueAttachmentVideo, DesktopIssueMaxVideoBytes, "VIDEO_TOO_LARGE"
	}
	if upload.SizeBytes > int64(limit) {
		return infraerrors.Newf(http.StatusRequestEntityTooLarge, reason, "each %s must be %d MiB or smaller", category, limit>>20)
	}
	attachment.Category = category
	attachment.MimeType = mimeType
	attachment.FileName = desktopIssueAttachmentFileName(upload.FileName, "attachment", ext)
	return nil
}

// The standard sniffer does not recognize MOV or MP4 with some ISO BMFF brands.
func detectDesktopIssueMediaMIME(header []byte, size int64, quickTimeHint bool) string {
	detected := http.DetectContentType(header)
	if len(header) < 8 || (detected != "application/octet-stream" && detected != "video/mp4") {
		return detected
	}
	boxSize := uint64(binary.BigEndian.Uint32(header[:4]))
	headerSize := 8
	switch boxSize {
	case 0:
		boxSize = uint64(size)
	case 1:
		if len(header) < 16 {
			return detected
		}
		boxSize = binary.BigEndian.Uint64(header[8:16])
		headerSize = 16
	}
	if boxSize < uint64(headerSize) || boxSize > uint64(size) {
		return detected
	}

	switch string(header[4:8]) {
	case "ftyp":
		if boxSize < uint64(headerSize+8) || len(header) < headerSize+8 {
			return detected
		}
		switch string(header[headerSize : headerSize+4]) {
		case "qt  ":
			return "video/quicktime"
		case "isom", "iso2", "iso3", "iso4", "iso5", "iso6", "iso7", "iso8", "iso9", "avc1", "M4V ", "dash":
			return "video/mp4"
		}
	case "moov", "mdat", "wide", "free", "skip", "pnot":
		if quickTimeHint {
			return "video/quicktime"
		}
	}
	return detected
}

func desktopIssueAttachmentFileName(raw, fallback, ext string) string {
	name := sanitizeAttachmentFileName(raw, fallback)
	originalExt := path.Ext(name)
	if desktopIssueExtensionMatches(strings.ToLower(originalExt), ext) {
		return name
	}
	stem := strings.TrimSuffix(name, originalExt)
	if stem == "" {
		stem = fallback
	}
	return stem + ext
}

func desktopIssueExtensionMatches(ext, canonical string) bool {
	return ext == canonical || (canonical == ".jpg" && ext == ".jpeg")
}

func sanitizeAttachmentFileName(raw, fallback string) string {
	base := path.Base(strings.ReplaceAll(strings.TrimSpace(raw), "\\", "/"))
	if base == "." || base == ".." || base == "/" {
		base = fallback
	}
	var b strings.Builder
	for _, r := range base {
		if r < 32 || r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' {
			b.WriteByte('_')
		} else {
			b.WriteRune(r)
		}
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return fallback
	}
	return truncateRunes(out, 180)
}

func truncateRunes(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}
