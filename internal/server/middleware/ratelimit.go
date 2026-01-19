package middleware

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"octopus/internal/server/resp"
	"octopus/internal/utils/log"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

var globalLimiter *rateFastSlowLimiter

var errQueueFull = errors.New("concurrency queue full")

type waiter struct {
	ch      chan struct{}
	granted bool
}

type queueLimiter struct {
	mu       sync.Mutex
	max      int
	active   int
	queue    []*waiter
	maxQueue int
}

func newQueueLimiter(max int, maxQueue int) *queueLimiter {
	if max <= 0 {
		return nil
	}
	return &queueLimiter{
		max:      max,
		maxQueue: maxQueue,
		queue:    make([]*waiter, 0),
	}
}

func (l *queueLimiter) acquire(ctx context.Context) error {
	l.mu.Lock()
	if l.active < l.max {
		l.active++
		l.mu.Unlock()
		return nil
	}
	if l.maxQueue <= 0 || len(l.queue) >= l.maxQueue {
		l.mu.Unlock()
		return errQueueFull
	}
	w := &waiter{ch: make(chan struct{})}
	l.queue = append(l.queue, w)
	l.mu.Unlock()

	select {
	case <-w.ch:
		return nil
	case <-ctx.Done():
		return l.cancelWait(w, ctx.Err())
	}
}

func (l *queueLimiter) cancelWait(w *waiter, cause error) error {
	l.mu.Lock()
	if w.granted {
		l.active--
		l.mu.Unlock()
		return cause
	}
	for i, item := range l.queue {
		if item == w {
			l.queue = append(l.queue[:i], l.queue[i+1:]...)
			l.mu.Unlock()
			return cause
		}
	}
	l.mu.Unlock()
	return cause
}

func (l *queueLimiter) release() {
	l.mu.Lock()
	if l.active > 0 {
		l.active--
	}
	if len(l.queue) > 0 {
		w := l.queue[0]
		l.queue = l.queue[1:]
		l.active++
		w.granted = true
		close(w.ch)
	}
	l.mu.Unlock()
}

type rateFastSlowLimiter struct {
	rateLimiter  *rate.Limiter
	fast         *queueLimiter
	slow         *queueLimiter
	migrateAfter time.Duration
	waitTimeout  time.Duration
}

// InitRateLimit 初始化全局并发限制器
// maxConcurrent: 最大并发请求数，0 表示不限制
func InitRateLimit(
	maxConcurrent int,
	fastMax int,
	slowMax int,
	migrateAfterSeconds int,
	ratePerSecond int,
	rateBurst int,
	maxQueue int,
	maxQueueWaitSeconds int,
) {
	if fastMax <= 0 && slowMax <= 0 {
		if maxConcurrent <= 0 && ratePerSecond <= 0 {
			globalLimiter = nil
			log.Infof("concurrency/rate limit disabled")
			return
		}
		if maxConcurrent > 0 {
			fastMax = maxConcurrent
			slowMax = maxConcurrent
			migrateAfterSeconds = 0
		}
	}

	var rateLimiter *rate.Limiter
	if ratePerSecond > 0 {
		if rateBurst <= 0 {
			rateBurst = ratePerSecond
		}
		rateLimiter = rate.NewLimiter(rate.Limit(ratePerSecond), rateBurst)
	}

	globalLimiter = &rateFastSlowLimiter{
		rateLimiter:  rateLimiter,
		fast:         newQueueLimiter(fastMax, maxQueue),
		slow:         newQueueLimiter(slowMax, maxQueue),
		migrateAfter: time.Duration(migrateAfterSeconds) * time.Second,
		waitTimeout:  time.Duration(maxQueueWaitSeconds) * time.Second,
	}

	log.Infof("limit enabled: rate %d/s burst %d, fast %d, slow %d, migrate %s, queue %d, max wait %s",
		ratePerSecond, rateBurst, fastMax, slowMax, globalLimiter.migrateAfter, maxQueue, globalLimiter.waitTimeout)
}

// RateLimit 返回并发限制中间件
func RateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 如果未启用限制，直接放行
		if globalLimiter == nil {
			c.Next()
			return
		}

		waitCtx := c.Request.Context()
		var cancel context.CancelFunc
		if globalLimiter.waitTimeout > 0 {
			waitCtx, cancel = context.WithTimeout(waitCtx, globalLimiter.waitTimeout)
			defer cancel()
		}

		// 1) 速率限制
		if globalLimiter.rateLimiter != nil {
			if err := globalLimiter.rateLimiter.Wait(waitCtx); err != nil {
				handleLimiterError(c, err)
				return
			}
		}

		// 2) 快池并发
		if globalLimiter.fast == nil && globalLimiter.slow == nil {
			c.Next()
			return
		}

		fast := globalLimiter.fast
		slow := globalLimiter.slow

		if fast == nil && slow != nil {
			if err := slow.acquire(waitCtx); err != nil {
				handleLimiterError(c, err)
				return
			}
			defer slow.release()
			c.Next()
			return
		}

		if err := fast.acquire(waitCtx); err != nil {
			handleLimiterError(c, err)
			return
		}

		type reqState struct {
			mu     sync.Mutex
			inSlow bool
			done   bool
		}
		state := &reqState{}
		doneCh := make(chan struct{})
		migrateCtx, migrateCancel := context.WithCancel(c.Request.Context())

		if slow != nil && globalLimiter.migrateAfter > 0 {
			go func() {
				timer := time.NewTimer(globalLimiter.migrateAfter)
				defer timer.Stop()
				select {
				case <-timer.C:
					if err := slow.acquire(migrateCtx); err != nil {
						return
					}
					state.mu.Lock()
					if state.done || state.inSlow {
						state.mu.Unlock()
						slow.release()
						return
					}
					state.inSlow = true
					state.mu.Unlock()
					fast.release()
				case <-doneCh:
					return
				case <-migrateCtx.Done():
					return
				}
			}()
		}

		defer func() {
			state.mu.Lock()
			state.done = true
			inSlow := state.inSlow
			state.mu.Unlock()
			close(doneCh)
			migrateCancel()
			if inSlow {
				slow.release()
				return
			}
			fast.release()
		}()

		c.Next()
	}
}

func handleLimiterError(c *gin.Context, err error) {
	if errors.Is(err, context.Canceled) {
		log.Infof("request canceled while waiting for limiter")
		return
	}
	if errors.Is(err, context.DeadlineExceeded) {
		resp.Error(c, http.StatusTooManyRequests, "request queue wait timeout")
		c.Abort()
		return
	}
	if errors.Is(err, errQueueFull) {
		resp.Error(c, http.StatusTooManyRequests, "request queue full")
		c.Abort()
		return
	}
	resp.Error(c, http.StatusTooManyRequests, "rate limit exceeded")
	c.Abort()
}
