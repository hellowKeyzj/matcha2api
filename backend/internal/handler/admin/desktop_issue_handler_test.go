package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type desktopIssueListRepo struct {
	service.DesktopIssueRepository
}

func (desktopIssueListRepo) List(_ context.Context, filter service.DesktopIssueListFilter) (*service.DesktopIssueListResult, error) {
	return &service.DesktopIssueListResult{
		Items: make([]*model.DesktopIssue, filter.PageSize),
		Total: 217,
	}, nil
}

func TestDesktopIssueListUsesEffectivePageSize(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewDesktopIssueHandler(service.NewDesktopIssueService(desktopIssueListRepo{}, nil, nil))
	router.GET("/issues", handler.List)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/issues?page=2&page_size=200", nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	var result struct {
		Data adminDesktopIssueListResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &result))
	require.Len(t, result.Data.Items, 100)
	require.Equal(t, 2, result.Data.Page)
	require.Equal(t, 100, result.Data.PageSize)
	require.Equal(t, 3, result.Data.Pages)
}
