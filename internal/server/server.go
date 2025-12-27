package server

import (
	"fmt"
	"net/http"

	"octopus/internal/conf"
	"octopus/internal/server/handlers"
	"octopus/internal/server/middleware"
	"octopus/internal/server/resp"
	"octopus/internal/server/router"
	"octopus/internal/utils/log"
	"octopus/static"
	"github.com/gin-gonic/gin"
)

var httpSrv http.Server

func Start() error {
	if conf.IsDebug() {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		resp.Error(c, http.StatusInternalServerError, resp.ErrInternalServer)
		c.Abort()
	}))

	if conf.IsDebug() {
		r.Use(middleware.Logger())
	}
	r.Use(middleware.Cors())
	r.Use(middleware.StaticEmbed("/", static.StaticFS))

	router.RegisterAll(r)

	// 注册 amp management 路由（需要直接访问 engine）
	handlers.RegisterAmpManagementRoutes(r)

	httpSrv.Addr = fmt.Sprintf("%s:%d", conf.AppConfig.Server.Host, conf.AppConfig.Server.Port)
	httpSrv.Handler = r
	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Errorf("http server listen and serve error: %v", err)
		}
	}()
	return nil
}

func Close() error {
	return httpSrv.Close()
}
