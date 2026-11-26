//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestDesktopIssueMutationPreservesConcurrentChanges(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	repo := NewDesktopIssueRepository(integrationDB)
	svc := service.NewDesktopIssueService(repo, nil, nil)
	id := createDesktopIssueForMutation(t, repo)

	// Queue all three operations on the same row before letting any write commit.
	lock := testTx(t)
	var lockedID int64
	require.NoError(t, lock.QueryRowContext(ctx, `SELECT id FROM desktop_issues WHERE id = $1 FOR NO KEY UPDATE`, id).Scan(&lockedID))

	done := make(chan error, 3)
	for _, note := range []string{"first note", "second note"} {
		go func() {
			_, err := svc.AddNote(ctx, service.AddDesktopIssueNoteRequest{IssueID: id, Body: note})
			done <- err
		}()
	}
	go func() {
		status := model.DesktopIssueStatusInProgress
		_, err := svc.Update(ctx, service.UpdateDesktopIssueRequest{IssueID: id, Status: &status})
		done <- err
	}()

	require.Eventually(t, func() bool {
		var waiting int
		err := integrationDB.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM pg_stat_activity
			WHERE datname = current_database() AND wait_event_type = 'Lock'
			  AND query LIKE '%desktop_issues%'`).Scan(&waiting)
		return err == nil && waiting == cap(done)
	}, 5*time.Second, 10*time.Millisecond, "all three mutations should reach the row before it is unlocked")
	require.NoError(t, lock.Commit())
	for range cap(done) {
		require.NoError(t, <-done)
	}

	detail, err := svc.GetDetail(ctx, id)
	require.NoError(t, err)
	require.Equal(t, 2, detail.Issue.CommentCount)
	require.Equal(t, model.DesktopIssueStatusInProgress, detail.Issue.Status)
	var notes []string
	for _, event := range detail.Events {
		if event.EventType == model.DesktopIssueEventNote {
			notes = append(notes, event.Body)
		}
	}
	require.ElementsMatch(t, []string{"first note", "second note"}, notes)
}

func TestDesktopIssueMutationStatusLifecycle(t *testing.T) {
	ctx := context.Background()
	repo := NewDesktopIssueRepository(integrationDB)
	svc := service.NewDesktopIssueService(repo, nil, nil)
	singleID := createDesktopIssueForMutation(t, repo)
	batchID := createDesktopIssueForMutation(t, repo)
	ids := []int64{batchID, singleID, batchID}
	resolved := model.DesktopIssueStatusResolved
	before, err := svc.Update(ctx, service.UpdateDesktopIssueRequest{IssueID: singleID, Status: &resolved})
	require.NoError(t, err)
	require.NotNil(t, before.Issue.ResolvedAt)
	count, err := svc.BatchUpdateStatus(ctx, ids, resolved, 0)
	require.NoError(t, err)
	require.EqualValues(t, 1, count)
	batchBefore, err := svc.GetDetail(ctx, batchID)
	require.NoError(t, err)
	require.NotNil(t, batchBefore.Issue.ResolvedAt)

	closed := model.DesktopIssueStatusClosed
	closedSingle, err := svc.AddNote(ctx, service.AddDesktopIssueNoteRequest{IssueID: singleID, Status: &closed})
	require.NoError(t, err)
	require.Equal(t, before.Issue.ResolvedAt, closedSingle.Issue.ResolvedAt)
	require.NotNil(t, closedSingle.Issue.ClosedAt)
	count, err = svc.BatchUpdateStatus(ctx, ids, closed, 0)
	require.NoError(t, err)
	require.EqualValues(t, 1, count)
	closedBatch, err := svc.GetDetail(ctx, batchID)
	require.NoError(t, err)
	require.Equal(t, batchBefore.Issue.ResolvedAt, closedBatch.Issue.ResolvedAt)
	require.NotNil(t, closedBatch.Issue.ClosedAt)
	require.Equal(t, map[string]any{"from": resolved, "to": closed, "batch": true}, closedBatch.Events[1].Metadata)

	count, err = svc.BatchUpdateStatus(ctx, ids, closed, 0)
	require.NoError(t, err)
	require.Zero(t, count)
	count, err = svc.BatchUpdateStatus(ctx, ids, model.DesktopIssueStatusOpen, 0)
	require.NoError(t, err)
	require.EqualValues(t, 2, count)
	for _, id := range []int64{singleID, batchID} {
		reopened, err := svc.GetDetail(ctx, id)
		require.NoError(t, err)
		require.Equal(t, model.DesktopIssueStatusOpen, reopened.Issue.Status)
		require.Nil(t, reopened.Issue.ResolvedAt)
		require.Nil(t, reopened.Issue.ClosedAt)
		require.Len(t, reopened.Events, 3)
	}
}

func createDesktopIssueForMutation(t *testing.T, repo service.DesktopIssueRepository) int64 {
	t.Helper()
	detail, err := repo.Create(context.Background(), &model.DesktopIssue{
		Title:    t.Name(),
		Kind:     model.DesktopIssueKindBug,
		Severity: model.DesktopIssueSeverityMajor,
		Status:   model.DesktopIssueStatusOpen,
	}, nil, nil)
	require.NoError(t, err)
	id := detail.Issue.ID
	t.Cleanup(func() {
		_, err := integrationDB.ExecContext(context.Background(), `DELETE FROM desktop_issues WHERE id = $1`, id)
		require.NoError(t, err)
	})
	return id
}
