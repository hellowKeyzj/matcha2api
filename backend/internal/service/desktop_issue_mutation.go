package service

import (
	"context"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/model"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type desktopIssueMutation struct {
	status       string
	severity     string
	labels       []string
	assigneeName *string
	fixVersion   *string
	note         string
	batch        bool
}

func (s *DesktopIssueService) Update(ctx context.Context, req UpdateDesktopIssueRequest) (*model.DesktopIssueDetail, error) {
	status, err := parseDesktopIssueMutationStatus(req.Status)
	if err != nil {
		return nil, err
	}
	change := desktopIssueMutation{status: status}
	if req.Severity != nil {
		change.severity = normalizeDesktopIssueSeverity(*req.Severity)
		if change.severity == "" {
			return nil, infraerrors.BadRequest("INVALID_ISSUE_SEVERITY", "invalid issue severity")
		}
	}
	if req.Labels != nil {
		change.labels = normalizeDesktopIssueLabels(*req.Labels)
	}
	change.assigneeName = normalizeDesktopIssueMutationText(req.AssigneeName, 128)
	change.fixVersion = normalizeDesktopIssueMutationText(req.FixVersion, 64)
	return s.mutateDesktopIssue(ctx, req.IssueID, req.ActorUserID, change)
}

func (s *DesktopIssueService) AddNote(ctx context.Context, req AddDesktopIssueNoteRequest) (*model.DesktopIssueDetail, error) {
	note := strings.TrimSpace(req.Body)
	if utf8.RuneCountInString(note) > DesktopIssueMaxNoteRunes {
		return nil, infraerrors.BadRequest("INVALID_NOTE_BODY", "note is too long")
	}
	if note == "" && req.Status == nil && req.FixVersion == nil {
		return nil, infraerrors.BadRequest("EMPTY_NOTE", "note body or state change is required")
	}
	status, err := parseDesktopIssueMutationStatus(req.Status)
	if err != nil {
		return nil, err
	}
	return s.mutateDesktopIssue(ctx, req.IssueID, req.ActorUserID, desktopIssueMutation{
		status:     status,
		fixVersion: normalizeDesktopIssueMutationText(req.FixVersion, 64),
		note:       note,
	})
}

func (s *DesktopIssueService) BatchUpdateStatus(ctx context.Context, ids []int64, status string, actorUserID int64) (int64, error) {
	nextStatus, err := parseDesktopIssueMutationStatus(&status)
	if err != nil {
		return 0, err
	}
	const maxBatchSize = 100
	unique := make([]int64, 0, min(len(ids), maxBatchSize))
	seen := make(map[int64]struct{}, cap(unique))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		if len(unique) == maxBatchSize {
			return 0, infraerrors.BadRequest("TOO_MANY_ISSUE_IDS", "at most 100 issues can be updated at once")
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	if len(unique) == 0 {
		return 0, infraerrors.BadRequest("EMPTY_ISSUE_IDS", "at least one issue id is required")
	}
	return s.mutateDesktopIssues(ctx, unique, actorUserID, desktopIssueMutation{status: nextStatus, batch: true})
}

func (s *DesktopIssueService) mutateDesktopIssue(ctx context.Context, id, actorUserID int64, change desktopIssueMutation) (*model.DesktopIssueDetail, error) {
	if _, err := s.mutateDesktopIssues(ctx, []int64{id}, actorUserID, change); err != nil {
		return nil, err
	}
	return s.repo.GetDetail(ctx, id)
}

func (s *DesktopIssueService) mutateDesktopIssues(ctx context.Context, ids []int64, actorUserID int64, change desktopIssueMutation) (int64, error) {
	if s == nil || s.repo == nil {
		return 0, infraerrors.ServiceUnavailable("DESKTOP_ISSUE_UNAVAILABLE", "feedback service unavailable")
	}
	email, username := s.resolveActorIdentity(ctx, actorUserID)
	actorName := displayIdentity(email, username, actorUserID)
	var actorID *int64
	if actorUserID > 0 {
		actorID = &actorUserID
	}
	return s.repo.Mutate(ctx, ids, func(issue *model.DesktopIssue, now time.Time) []*model.DesktopIssueEvent {
		return change.apply(issue, now, actorID, actorName)
	})
}

func (change desktopIssueMutation) apply(issue *model.DesktopIssue, now time.Time, actorID *int64, actorName string) []*model.DesktopIssueEvent {
	var events []*model.DesktopIssueEvent
	addEvent := func(eventType, body string, metadata map[string]any) {
		events = append(events, &model.DesktopIssueEvent{
			IssueID:     issue.ID,
			EventType:   eventType,
			ActorUserID: actorID,
			ActorName:   actorName,
			Body:        body,
			Metadata:    metadata,
		})
	}
	setField := func(eventType string, field *string, next string) {
		if *field != next {
			addEvent(eventType, "", map[string]any{"from": *field, "to": next})
			*field = next
		}
	}
	if change.severity != "" {
		setField(model.DesktopIssueEventSeverityChanged, &issue.Severity, change.severity)
	}
	if change.labels != nil && !slices.Equal(issue.Labels, change.labels) {
		var added, removed []string
		for _, label := range change.labels {
			if !slices.Contains(issue.Labels, label) {
				added = append(added, label)
			}
		}
		for _, label := range issue.Labels {
			if !slices.Contains(change.labels, label) {
				removed = append(removed, label)
			}
		}
		issue.Labels = change.labels
		addEvent(model.DesktopIssueEventLabelsChanged, "", map[string]any{"added": added, "removed": removed, "labels": change.labels})
	}
	if change.assigneeName != nil {
		setField(model.DesktopIssueEventAssigneeChanged, &issue.AssigneeName, *change.assigneeName)
	}
	if change.fixVersion != nil {
		setField(model.DesktopIssueEventFixVersionChanged, &issue.FixVersion, *change.fixVersion)
	}
	if change.note != "" {
		metadata := map[string]any{}
		if change.fixVersion != nil && *change.fixVersion != "" {
			metadata["fix_version"] = *change.fixVersion
		}
		addEvent(model.DesktopIssueEventNote, change.note, metadata)
		issue.CommentCount++
	}
	if change.status != "" && change.status != issue.Status {
		metadata := map[string]any{"from": issue.Status, "to": change.status}
		if change.batch {
			metadata["batch"] = true
		}
		issue.Status = change.status
		switch change.status {
		case model.DesktopIssueStatusResolved:
			issue.ResolvedAt, issue.ClosedAt = &now, nil
		case model.DesktopIssueStatusClosed:
			issue.ClosedAt = &now
		default:
			issue.ResolvedAt, issue.ClosedAt = nil, nil
		}
		addEvent(model.DesktopIssueEventStatusChanged, "", metadata)
	}
	return events
}

func parseDesktopIssueMutationStatus(status *string) (string, error) {
	if status == nil {
		return "", nil
	}
	value := strings.ToLower(strings.TrimSpace(*status))
	switch value {
	case model.DesktopIssueStatusOpen, model.DesktopIssueStatusInProgress, model.DesktopIssueStatusResolved, model.DesktopIssueStatusClosed:
		return value, nil
	default:
		return "", infraerrors.BadRequest("INVALID_ISSUE_STATUS", "invalid issue status")
	}
}

func normalizeDesktopIssueMutationText(value *string, maxRunes int) *string {
	if value == nil {
		return nil
	}
	clean := truncateRunes(strings.TrimSpace(*value), maxRunes)
	return &clean
}

func normalizeDesktopIssueLabels(labels []string) []string {
	out := make([]string, 0, min(len(labels), DesktopIssueMaxLabels))
	for _, raw := range labels {
		label := strings.ToLower(strings.TrimSpace(raw))
		if label == "" || utf8.RuneCountInString(label) > DesktopIssueMaxLabelRunes || slices.Contains(out, label) {
			continue
		}
		out = append(out, label)
		if len(out) == DesktopIssueMaxLabels {
			break
		}
	}
	return out
}
