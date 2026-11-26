package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type desktopIssueRepository struct {
	db *sql.DB
}

var _ service.DesktopIssueRepository = (*desktopIssueRepository)(nil)

func NewDesktopIssueRepository(db *sql.DB) service.DesktopIssueRepository {
	return &desktopIssueRepository{db: db}
}

func (r *desktopIssueRepository) Create(
	ctx context.Context,
	issue *model.DesktopIssue,
	attachments []*model.DesktopIssueAttachment,
	initialEvent *model.DesktopIssueEvent,
) (*model.DesktopIssueDetail, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	labelsJSON, err := jsonStringSliceParam(issue.Labels)
	if err != nil {
		return nil, err
	}
	metaJSON, err := jsonMapParam(issue.ClientMeta)
	if err != nil {
		return nil, err
	}

	created, err := scanDesktopIssue(tx.QueryRowContext(ctx, `
		INSERT INTO desktop_issues (
			reporter_user_id, reporter_email, reporter_username,
			title, description, kind, severity, status, labels,
			assignee_user_id, assignee_name, fix_version,
			app_version, client_os, client_arch, client_route, client_locale, client_meta,
			has_diagnostics, image_count, video_count, comment_count
		)
		VALUES (
			$1, $2, $3,
			$4, $5, $6, $7, $8, $9::jsonb,
			$10, $11, $12,
			$13, $14, $15, $16, $17, $18::jsonb,
			$19, $20, $21, $22
		)
		RETURNING
			id, reporter_user_id, reporter_email, reporter_username,
			title, description, kind, severity, status, labels,
			assignee_user_id, assignee_name, fix_version,
			app_version, client_os, client_arch, client_route, client_locale, client_meta,
			has_diagnostics, image_count, video_count, comment_count,
			created_at, updated_at, resolved_at, closed_at`,
		nullableInt64(issue.ReporterUserID), issue.ReporterEmail, issue.ReporterUsername,
		issue.Title, issue.Description, issue.Kind, issue.Severity, issue.Status, labelsJSON,
		nullableInt64(issue.AssigneeUserID), issue.AssigneeName, issue.FixVersion,
		issue.AppVersion, issue.ClientOS, issue.ClientArch, issue.ClientRoute, issue.ClientLocale, metaJSON,
		issue.HasDiagnostics, issue.ImageCount, issue.VideoCount, issue.CommentCount,
	))
	if err != nil {
		return nil, err
	}

	createdAttachments := make([]*model.DesktopIssueAttachment, 0, len(attachments))
	for _, att := range attachments {
		row, err := scanDesktopIssueAttachment(tx.QueryRowContext(ctx, `
			INSERT INTO desktop_issue_attachments (
				issue_id, category, file_name, mime_type, size_bytes, sha256, object_key
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			RETURNING id, issue_id, category, file_name, mime_type, size_bytes, sha256, object_key, created_at`,
			created.ID, att.Category, att.FileName, att.MimeType, att.SizeBytes, att.SHA256, att.ObjectKey,
		))
		if err != nil {
			return nil, err
		}
		createdAttachments = append(createdAttachments, row)
	}

	events := make([]*model.DesktopIssueEvent, 0, 1)
	if initialEvent != nil {
		initialEvent.IssueID = created.ID
		ev, err := insertDesktopIssueEventTx(ctx, tx, initialEvent)
		if err != nil {
			return nil, err
		}
		events = append(events, ev)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &model.DesktopIssueDetail{
		Issue:              created,
		Attachments:        createdAttachments,
		Events:             events,
		ReporterIssueCount: 1,
	}, nil
}

func (r *desktopIssueRepository) List(ctx context.Context, filter service.DesktopIssueListFilter) (*service.DesktopIssueListResult, error) {
	// Status counts across all issues (matching non-status filters so counts reflect active filters).
	baseWhere, baseArgs := buildDesktopIssueWhere(filter, false)
	counts, err := r.queryStatusCounts(ctx, baseWhere, baseArgs)
	if err != nil {
		return nil, err
	}

	where, args := buildDesktopIssueWhere(filter, true)
	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM desktop_issues WHERE `+where, args...).Scan(&total); err != nil {
		return nil, err
	}

	offset := (filter.Page - 1) * filter.PageSize
	listArgs := append(append([]any{}, args...), filter.PageSize, offset)
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			id, reporter_user_id, reporter_email, reporter_username,
			title, description, kind, severity, status, labels,
			assignee_user_id, assignee_name, fix_version,
			app_version, client_os, client_arch, client_route, client_locale, client_meta,
			has_diagnostics, image_count, video_count, comment_count,
			created_at, updated_at, resolved_at, closed_at
		FROM desktop_issues
		WHERE `+where+`
		ORDER BY created_at DESC, id DESC
		LIMIT $`+strconv.Itoa(len(args)+1)+` OFFSET $`+strconv.Itoa(len(args)+2),
		listArgs...,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]*model.DesktopIssue, 0, filter.PageSize)
	for rows.Next() {
		item, err := scanDesktopIssue(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &service.DesktopIssueListResult{
		Items:  items,
		Total:  total,
		Counts: counts,
	}, nil
}

func (r *desktopIssueRepository) GetDetail(ctx context.Context, id int64) (*model.DesktopIssueDetail, error) {
	issue, err := scanDesktopIssue(r.db.QueryRowContext(ctx, `
		SELECT
			id, reporter_user_id, reporter_email, reporter_username,
			title, description, kind, severity, status, labels,
			assignee_user_id, assignee_name, fix_version,
			app_version, client_os, client_arch, client_route, client_locale, client_meta,
			has_diagnostics, image_count, video_count, comment_count,
			created_at, updated_at, resolved_at, closed_at
		FROM desktop_issues
		WHERE id = $1`, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrDesktopIssueNotFound
		}
		return nil, err
	}

	attachments, err := r.listAttachments(ctx, id)
	if err != nil {
		return nil, err
	}
	events, err := r.listEvents(ctx, id)
	if err != nil {
		return nil, err
	}

	var reporterCount int64 = 1
	if issue.ReporterUserID != nil && *issue.ReporterUserID > 0 {
		_ = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM desktop_issues WHERE reporter_user_id = $1`, *issue.ReporterUserID).Scan(&reporterCount)
	} else if issue.ReporterEmail != "" {
		_ = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM desktop_issues WHERE reporter_email = $1`, issue.ReporterEmail).Scan(&reporterCount)
	}

	return &model.DesktopIssueDetail{
		Issue:              issue,
		Attachments:        attachments,
		Events:             events,
		ReporterIssueCount: reporterCount,
	}, nil
}

func (r *desktopIssueRepository) GetAttachment(ctx context.Context, issueID, attachmentID int64) (*model.DesktopIssueAttachment, error) {
	att, err := scanDesktopIssueAttachment(r.db.QueryRowContext(ctx, `
		SELECT id, issue_id, category, file_name, mime_type, size_bytes, sha256, object_key, created_at
		FROM desktop_issue_attachments
		WHERE id = $1 AND issue_id = $2`, attachmentID, issueID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrDesktopIssueAttachmentNotFound
		}
		return nil, err
	}
	return att, nil
}

type desktopIssueWrite struct {
	ID           int64                      `json:"id"`
	Status       string                     `json:"status"`
	Severity     string                     `json:"severity"`
	Labels       []string                   `json:"labels"`
	AssigneeName string                     `json:"assignee_name"`
	FixVersion   string                     `json:"fix_version"`
	CommentCount int                        `json:"comment_count"`
	ResolvedAt   *time.Time                 `json:"resolved_at"`
	ClosedAt     *time.Time                 `json:"closed_at"`
	Events       []*model.DesktopIssueEvent `json:"events"`
}

// Mutate locks the editable fields in ID order before applying any changes.
// A change and its events either commit together or are both discarded.
func (r *desktopIssueRepository) Mutate(
	ctx context.Context,
	ids []int64,
	apply func(*model.DesktopIssue, time.Time) []*model.DesktopIssueEvent,
) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	issues, err := lockDesktopIssues(ctx, tx, ids)
	if err != nil {
		return 0, err
	}
	// Use the time after lock acquisition so waiting transactions do not record
	// changes as if they occurred before the mutations they waited for.
	now := time.Now().UTC()
	writes := make([]desktopIssueWrite, 0, len(issues))
	for i := range issues {
		issue := &issues[i]
		events := apply(issue, now)
		if len(events) == 0 {
			continue
		}
		writes = append(writes, desktopIssueWrite{
			ID:           issue.ID,
			Status:       issue.Status,
			Severity:     issue.Severity,
			Labels:       issue.Labels,
			AssigneeName: issue.AssigneeName,
			FixVersion:   issue.FixVersion,
			CommentCount: issue.CommentCount,
			ResolvedAt:   issue.ResolvedAt,
			ClosedAt:     issue.ClosedAt,
			Events:       events,
		})
	}
	if len(writes) > 0 {
		payload, err := json.Marshal(writes)
		if err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx, `
			WITH updated AS (
				UPDATE desktop_issues AS issue
				SET status = change.status, severity = change.severity, labels = change.labels,
				    assignee_name = change.assignee_name, fix_version = change.fix_version,
				    comment_count = change.comment_count, resolved_at = change.resolved_at,
				    closed_at = change.closed_at, updated_at = $2
				FROM jsonb_to_recordset($1::jsonb) AS change(
					id bigint, status text, severity text, labels jsonb, assignee_name text, fix_version text,
					comment_count integer, resolved_at timestamptz, closed_at timestamptz, events jsonb
				)
				WHERE issue.id = change.id
				RETURNING issue.id, change.events
			)
			INSERT INTO desktop_issue_events (issue_id, event_type, actor_user_id, actor_name, body, metadata, created_at)
			SELECT updated.id, event.data->>'event_type', (event.data->>'actor_user_id')::bigint,
			       event.data->>'actor_name', event.data->>'body', event.data->'metadata', $2
			FROM updated
			CROSS JOIN LATERAL jsonb_array_elements(updated.events) WITH ORDINALITY AS event(data, position)
			ORDER BY updated.id, event.position`, string(payload), now,
		); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return int64(len(writes)), nil
}

func lockDesktopIssues(ctx context.Context, tx *sql.Tx, ids []int64) ([]model.DesktopIssue, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, status, severity, labels, assignee_name, fix_version, comment_count, resolved_at, closed_at
		FROM desktop_issues WHERE id = ANY($1) ORDER BY id FOR NO KEY UPDATE`, pq.Array(ids))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	issues := make([]model.DesktopIssue, 0, len(ids))
	for rows.Next() {
		var issue model.DesktopIssue
		var labels []byte
		if err := rows.Scan(&issue.ID, &issue.Status, &issue.Severity, &labels, &issue.AssigneeName,
			&issue.FixVersion, &issue.CommentCount, &issue.ResolvedAt, &issue.ClosedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(labels, &issue.Labels); err != nil {
			return nil, err
		}
		if issue.Labels == nil {
			issue.Labels = []string{}
		}
		issues = append(issues, issue)
	}
	return issues, rows.Err()
}

func (r *desktopIssueRepository) queryStatusCounts(ctx context.Context, where string, args []any) (model.DesktopIssueStatusCounts, error) {
	var counts model.DesktopIssueStatusCounts
	rows, err := r.db.QueryContext(ctx, `
		SELECT status, COUNT(*)
		FROM desktop_issues
		WHERE `+where+`
		GROUP BY status`, args...)
	if err != nil {
		return counts, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var status string
		var cnt int64
		if err := rows.Scan(&status, &cnt); err != nil {
			return counts, err
		}
		counts.All += cnt
		switch status {
		case model.DesktopIssueStatusOpen:
			counts.Open = cnt
		case model.DesktopIssueStatusInProgress:
			counts.InProgress = cnt
		case model.DesktopIssueStatusResolved:
			counts.Resolved = cnt
		case model.DesktopIssueStatusClosed:
			counts.Closed = cnt
		}
	}
	return counts, rows.Err()
}

func (r *desktopIssueRepository) listAttachments(ctx context.Context, issueID int64) ([]*model.DesktopIssueAttachment, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, issue_id, category, file_name, mime_type, size_bytes, sha256, object_key, created_at
		FROM desktop_issue_attachments
		WHERE issue_id = $1
		ORDER BY id ASC`, issueID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]*model.DesktopIssueAttachment, 0)
	for rows.Next() {
		att, err := scanDesktopIssueAttachment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, att)
	}
	return out, rows.Err()
}

func (r *desktopIssueRepository) listEvents(ctx context.Context, issueID int64) ([]*model.DesktopIssueEvent, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, issue_id, event_type, actor_user_id, actor_name, body, metadata, created_at
		FROM desktop_issue_events
		WHERE issue_id = $1
		ORDER BY created_at ASC, id ASC`, issueID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]*model.DesktopIssueEvent, 0)
	for rows.Next() {
		ev, err := scanDesktopIssueEvent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, ev)
	}
	return out, rows.Err()
}

func buildDesktopIssueWhere(filter service.DesktopIssueListFilter, includeStatus bool) (string, []any) {
	clauses := []string{"1 = 1"}
	args := make([]any, 0, 6)

	if includeStatus && filter.Status != "" && filter.Status != "all" {
		args = append(args, filter.Status)
		clauses = append(clauses, fmt.Sprintf("status = $%d", len(args)))
	}
	if filter.Kind != "" && filter.Kind != "all" {
		args = append(args, filter.Kind)
		clauses = append(clauses, fmt.Sprintf("kind = $%d", len(args)))
	}
	if filter.Severity != "" && filter.Severity != "all" {
		args = append(args, filter.Severity)
		clauses = append(clauses, fmt.Sprintf("severity = $%d", len(args)))
	}
	if filter.HasDiagnostics != nil {
		args = append(args, *filter.HasDiagnostics)
		clauses = append(clauses, fmt.Sprintf("has_diagnostics = $%d", len(args)))
	}
	if rep := strings.TrimSpace(filter.Reporter); rep != "" {
		args = append(args, rep)
		clauses = append(clauses, fmt.Sprintf("reporter_email = $%d", len(args)))
	}
	if q := strings.TrimSpace(filter.Search); q != "" {
		cleanNum := strings.TrimPrefix(q, "#")
		if idNum, err := strconv.ParseInt(cleanNum, 10, 64); err == nil && idNum > 0 {
			args = append(args, idNum)
			idIdx := len(args)
			args = append(args, "%"+q+"%")
			likeIdx := len(args)
			clauses = append(clauses, fmt.Sprintf(
				"(id = $%d OR title ILIKE $%d OR reporter_email ILIKE $%d OR app_version ILIKE $%d OR fix_version ILIKE $%d OR labels::text ILIKE $%d)",
				idIdx, likeIdx, likeIdx, likeIdx, likeIdx, likeIdx,
			))
		} else {
			args = append(args, "%"+q+"%")
			idx := len(args)
			clauses = append(clauses, fmt.Sprintf(
				"(title ILIKE $%d OR description ILIKE $%d OR reporter_email ILIKE $%d OR reporter_username ILIKE $%d OR app_version ILIKE $%d OR fix_version ILIKE $%d OR labels::text ILIKE $%d)",
				idx, idx, idx, idx, idx, idx, idx,
			))
		}
	}
	return strings.Join(clauses, " AND "), args
}

func insertDesktopIssueEventTx(ctx context.Context, tx *sql.Tx, ev *model.DesktopIssueEvent) (*model.DesktopIssueEvent, error) {
	metaJSON, err := jsonMapParam(ev.Metadata)
	if err != nil {
		return nil, err
	}
	return scanDesktopIssueEvent(tx.QueryRowContext(ctx, `
		INSERT INTO desktop_issue_events (issue_id, event_type, actor_user_id, actor_name, body, metadata)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb)
		RETURNING id, issue_id, event_type, actor_user_id, actor_name, body, metadata, created_at`,
		ev.IssueID, ev.EventType, nullableInt64(ev.ActorUserID), ev.ActorName, ev.Body, metaJSON,
	))
}

func scanDesktopIssue(scanner interface{ Scan(...any) error }) (*model.DesktopIssue, error) {
	var item model.DesktopIssue
	var reporterUID, assigneeUID sql.NullInt64
	var labelsRaw, metaRaw []byte
	var resolvedAt, closedAt sql.NullTime

	err := scanner.Scan(
		&item.ID, &reporterUID, &item.ReporterEmail, &item.ReporterUsername,
		&item.Title, &item.Description, &item.Kind, &item.Severity, &item.Status, &labelsRaw,
		&assigneeUID, &item.AssigneeName, &item.FixVersion,
		&item.AppVersion, &item.ClientOS, &item.ClientArch, &item.ClientRoute, &item.ClientLocale, &metaRaw,
		&item.HasDiagnostics, &item.ImageCount, &item.VideoCount, &item.CommentCount,
		&item.CreatedAt, &item.UpdatedAt, &resolvedAt, &closedAt,
	)
	if err != nil {
		return nil, err
	}
	if reporterUID.Valid {
		v := reporterUID.Int64
		item.ReporterUserID = &v
	}
	if assigneeUID.Valid {
		v := assigneeUID.Int64
		item.AssigneeUserID = &v
	}
	if resolvedAt.Valid {
		t := resolvedAt.Time
		item.ResolvedAt = &t
	}
	if closedAt.Valid {
		t := closedAt.Time
		item.ClosedAt = &t
	}
	if len(labelsRaw) > 0 {
		_ = json.Unmarshal(labelsRaw, &item.Labels)
	}
	if item.Labels == nil {
		item.Labels = []string{}
	}
	if len(metaRaw) > 0 {
		_ = json.Unmarshal(metaRaw, &item.ClientMeta)
	}
	if item.ClientMeta == nil {
		item.ClientMeta = map[string]any{}
	}
	return &item, nil
}

func scanDesktopIssueAttachment(scanner interface{ Scan(...any) error }) (*model.DesktopIssueAttachment, error) {
	var att model.DesktopIssueAttachment
	if err := scanner.Scan(
		&att.ID, &att.IssueID, &att.Category, &att.FileName,
		&att.MimeType, &att.SizeBytes, &att.SHA256, &att.ObjectKey, &att.CreatedAt,
	); err != nil {
		return nil, err
	}
	return &att, nil
}

func scanDesktopIssueEvent(scanner interface{ Scan(...any) error }) (*model.DesktopIssueEvent, error) {
	var ev model.DesktopIssueEvent
	var actorUID sql.NullInt64
	var metaRaw []byte
	if err := scanner.Scan(
		&ev.ID, &ev.IssueID, &ev.EventType, &actorUID, &ev.ActorName, &ev.Body, &metaRaw, &ev.CreatedAt,
	); err != nil {
		return nil, err
	}
	if actorUID.Valid {
		v := actorUID.Int64
		ev.ActorUserID = &v
	}
	if len(metaRaw) > 0 {
		_ = json.Unmarshal(metaRaw, &ev.Metadata)
	}
	if ev.Metadata == nil {
		ev.Metadata = map[string]any{}
	}
	return &ev, nil
}

func jsonStringSliceParam(items []string) (string, error) {
	if items == nil {
		return "[]", nil
	}
	raw, err := json.Marshal(items)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}
