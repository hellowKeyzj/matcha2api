package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func consumePackageMeteringHeader(c *gin.Context) service.PackageMeteringUsage {
	if c == nil || c.Request == nil {
		return service.PackageMeteringUsage{}
	}
	usage := service.ParsePackageMeteringRequestHeader(c.Request.Header)
	service.StripMatchaHeaders(c.Request.Header)
	return usage
}
