package router

import (
	"chat-demo/api"
	"chat-demo/service"
	"github.com/gin-gonic/gin"
)

func NewRouter() *gin.Engine {
	r := gin.Default()
	r.Use(gin.Recovery(), gin.Logger())
	// Recovery 中间件会恢复 (recovers) 任何恐慌（panics) 如果存在恐慌  中间件将写入500 中间件很有必要
	//Logger
	v1 := r.Group("/")
	{
		v1.GET("ping", func(c *gin.Context) {
			c.JSON(200, "success")
		})
		v1.POST("user/register", api.UserRegister)
		v1.GET("ws", service.Handler)
	}
	return r
}
