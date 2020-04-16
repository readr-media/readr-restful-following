package router

import "github.com/gin-gonic/gin"

type RouterHandler interface {
	SetRoutes(router *gin.Engine)
}
