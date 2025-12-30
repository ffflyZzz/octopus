package middleware

import (
	"context"
	"errors"
	"net/http"

	"octopus/internal/server/resp"
	"octopus/internal/utils/log"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// globalLimiter 全局限流器
var globalLimiter *rate.Limiter

// InitRateLimit 初始化全局限流器
// rps: 每秒允许的请求数，0 表示不限流
func InitRateLimit(rps int) {
	if rps <= 0 {
		globalLimiter = nil
		log.Infof("rate limit disabled")
		return
	}
	// burst 设置为 rps，允许短时间突发
	globalLimiter = rate.NewLimiter(rate.Limit(rps), rps)
	log.Infof("rate limit enabled: %d requests per second", rps)
}

// RateLimit 返回限流中间件
func RateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 如果未启用限流，直接放行
		if globalLimiter == nil {
			c.Next()
			return
		}

		// 等待获取 token（阻塞直到可以通过）
		err := globalLimiter.Wait(c.Request.Context())
		if err != nil {
			if errors.Is(err, context.Canceled) {
				log.Infof("request canceled while waiting for rate limit")
				return
			}
			if errors.Is(err, context.DeadlineExceeded) {
				resp.Error(c, http.StatusRequestTimeout, "rate limit wait timeout")
				c.Abort()
				return
			}
			resp.Error(c, http.StatusTooManyRequests, "rate limit exceeded")
			c.Abort()
			return
		}

		c.Next()
	}
}
