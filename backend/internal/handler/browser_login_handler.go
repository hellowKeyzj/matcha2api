package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type BrowserLoginHandler struct {
	service *service.BrowserLoginService
}

func NewBrowserLoginHandler(browserLoginService *service.BrowserLoginService) *BrowserLoginHandler {
	return &BrowserLoginHandler{service: browserLoginService}
}

func (h *BrowserLoginHandler) Issue(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	ticket, err := h.service.Issue(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, ticket)
}

func (h *BrowserLoginHandler) Exchange(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	var req struct {
		Ticket string `json:"ticket" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, service.ErrInvalidBrowserLoginTicket)
		return
	}
	pair, user, err := h.service.Exchange(c.Request.Context(), req.Ticket)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	middleware2.SetAuditActor(c, user.ID, user.Email)
	response.Success(c, AuthResponse{
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		ExpiresIn:    pair.ExpiresIn,
		TokenType:    "Bearer",
		User:         dto.UserFromService(user),
	})
}
