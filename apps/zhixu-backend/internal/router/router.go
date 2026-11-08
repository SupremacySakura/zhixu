package router

import (
    "fmt"
    "net/http"

    "github.com/gin-gonic/gin"
    swaggerFiles "github.com/swaggo/files"
    ginSwagger "github.com/swaggo/gin-swagger"

    _ "zhixu-backend/docs"
    "zhixu-backend/internal/config"
    "zhixu-backend/internal/service"
    "zhixu-backend/internal/middleware"
)

func SetUpRouter(cfg *config.Config) *gin.Engine {
    r := gin.Default()
    // 只信任配置内的代理来源，否则客户端可伪造 X-Forwarded-For 影响限流与日志。
    // 空列表表示不信任任何代理，ClientIP 退化为直连地址。
    proxies := cfg.GetTrustedProxies()
    if proxies == nil {
        proxies = []string{}
    }
    if err := r.SetTrustedProxies(proxies); err != nil {
        panic(fmt.Sprintf("无效的 TRUSTED_PROXIES 配置: %v", err))
    }
    // 注册跨域、限流和日志中间件
    r.Use(middleware.CORSMiddleware(cfg))
    r.Use(middleware.RateLimitMiddleware())
    r.Use(middleware.LoggerMiddleware())
	RegisterUserRouter(r)
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.HEAD("/healthz", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	RegisterFileRouter(r, service.FileServiceOptions{
		ChunkStoragePath: cfg.ChunkStoragePath,
		FileStoragePath:  cfg.FileStoragePath,
	})
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	return r
}
