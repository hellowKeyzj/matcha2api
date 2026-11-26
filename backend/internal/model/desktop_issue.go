package model

import "time"

const (
	DesktopIssueKindBug        = "bug"
	DesktopIssueKindFeature    = "feature"
	DesktopIssueKindExperience = "experience"

	DesktopIssueSeverityBlocker = "blocker"
	DesktopIssueSeverityMajor   = "major"
	DesktopIssueSeverityMinor   = "minor"

	DesktopIssueStatusOpen       = "open"
	DesktopIssueStatusInProgress = "in_progress"
	DesktopIssueStatusResolved   = "resolved"
	DesktopIssueStatusClosed     = "closed"

	DesktopIssueAttachmentImage       = "image"
	DesktopIssueAttachmentVideo       = "video"
	DesktopIssueAttachmentDiagnostics = "diagnostics"

	DesktopIssueEventCreated           = "created"
	DesktopIssueEventNote              = "note"
	DesktopIssueEventStatusChanged     = "status_changed"
	DesktopIssueEventLabelsChanged     = "labels_changed"
	DesktopIssueEventAssigneeChanged   = "assignee_changed"
	DesktopIssueEventSeverityChanged   = "severity_changed"
	DesktopIssueEventFixVersionChanged = "fix_version_changed"
)

type DesktopIssue struct {
	ID               int64          `json:"id"`
	ReporterUserID   *int64         `json:"reporter_user_id,omitempty"`
	ReporterEmail    string         `json:"reporter_email"`
	ReporterUsername string         `json:"reporter_username"`
	Title            string         `json:"title"`
	Description      string         `json:"description"`
	Kind             string         `json:"kind"`
	Severity         string         `json:"severity"`
	Status           string         `json:"status"`
	Labels           []string       `json:"labels"`
	AssigneeUserID   *int64         `json:"assignee_user_id,omitempty"`
	AssigneeName     string         `json:"assignee_name"`
	FixVersion       string         `json:"fix_version"`
	AppVersion       string         `json:"app_version"`
	ClientOS         string         `json:"client_os"`
	ClientArch       string         `json:"client_arch"`
	ClientRoute      string         `json:"client_route"`
	ClientLocale     string         `json:"client_locale"`
	ClientMeta       map[string]any `json:"client_meta"`
	HasDiagnostics   bool           `json:"has_diagnostics"`
	ImageCount       int            `json:"image_count"`
	VideoCount       int            `json:"video_count"`
	CommentCount     int            `json:"comment_count"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	ResolvedAt       *time.Time     `json:"resolved_at,omitempty"`
	ClosedAt         *time.Time     `json:"closed_at,omitempty"`
}

type DesktopIssueAttachment struct {
	ID        int64     `json:"id"`
	IssueID   int64     `json:"issue_id"`
	Category  string    `json:"category"`
	FileName  string    `json:"file_name"`
	MimeType  string    `json:"mime_type"`
	SizeBytes int64     `json:"size_bytes"`
	SHA256    string    `json:"sha256"`
	ObjectKey string    `json:"-"`
	CreatedAt time.Time `json:"created_at"`
}

type DesktopIssueEvent struct {
	ID          int64          `json:"id"`
	IssueID     int64          `json:"issue_id"`
	EventType   string         `json:"event_type"`
	ActorUserID *int64         `json:"actor_user_id,omitempty"`
	ActorName   string         `json:"actor_name"`
	Body        string         `json:"body"`
	Metadata    map[string]any `json:"metadata"`
	CreatedAt   time.Time      `json:"created_at"`
}

type DesktopIssueDetail struct {
	Issue              *DesktopIssue             `json:"issue"`
	Attachments        []*DesktopIssueAttachment `json:"attachments"`
	Events             []*DesktopIssueEvent      `json:"events"`
	ReporterIssueCount int64                     `json:"reporter_issue_count"`
}

type DesktopIssueStatusCounts struct {
	All        int64 `json:"all"`
	Open       int64 `json:"open"`
	InProgress int64 `json:"in_progress"`
	Resolved   int64 `json:"resolved"`
	Closed     int64 `json:"closed"`
}

type DesktopIssueManifestEntry struct {
	Name      string `json:"name"`
	SizeBytes uint64 `json:"size_bytes"`
}
