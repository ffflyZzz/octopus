package middleware

import (
	"context"
	"errors"
	"net/http"

	"octopus/internal/server/resp"
	"octopus/internal/utils/log"

	"github.com/gin-gonic/gin"
)

// globalSemaphore 全局并发限制器
var globalSemaphore chan struct{}

// InitRateLimit 初始化全局并发限制器
// maxConcurrent: 最大并发请求数，0 表示不限制
func InitRateLimit(maxConcurrent int) {
	if maxConcurrent <= 0 {
		globalSemaphore = nil
		log.Infof("concurrency limit disabled")
		return
	}
	globalSemaphore = make(chan struct{}, maxConcurrent)
	log.Infof("concurrency limit enabled: max %d concurrent requests", maxConcurrent)
}

// RateLimit 返回并发限制中间件
func RateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 如果未启用限制，直接放行
		if globalSemaphore == nil {
			c.Next()
			return
		}

		// 尝试获取并发令牌（阻塞直到可以通过）
		select {
		case globalSemaphore <- struct{}{}:
			// 获取成功，请求结束时释放
			defer func() { <-globalSemaphore }()
			c.Next()
		case <-c.Request.Context().Done():
			// 请求被取消或超时
			err := c.Request.Context().Err()
			if errors.Is(err, context.Canceled) {
				log.Infof("request canceled while waiting for concurrency slot")
				return
			}
			if errors.Is(err, context.DeadlineExceeded) {
				resp.Error(c, http.StatusRequestTimeout, "concurrency wait timeout")
				c.Abort()
				return
			}
			resp.Error(c, http.StatusTooManyRequests, "concurrency limit exceeded")
			c.Abort()
		}
	}
}
