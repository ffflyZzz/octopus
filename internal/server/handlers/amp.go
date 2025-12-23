package handlers

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bestruirui/octopus/internal/conf"
	"github.com/bestruirui/octopus/internal/relay"
	"github.com/bestruirui/octopus/internal/server/middleware"
	"github.com/bestruirui/octopus/internal/server/router"
	"github.com/bestruirui/octopus/internal/transformer/inbound"
	"github.com/bestruirui/octopus/internal/utils/log"
	"github.com/gin-gonic/gin"
)

// Free tier request constants
const (
	internalAPIPath            = "/api/internal"
	webSearchQuery             = "webSearch2"
	extractWebPageContentQuery = "extractWebPageContent"
)

// Regex to match isFreeTierRequest: false
var isFreeTierRequestRegex = regexp.MustCompile(`"isFreeTierRequest"\s*:\s*false`)

// TODO(bedrock-ttl): 当 Bedrock 支持 cache_control.ttl 后，删除以下正则和相关过滤逻辑
// Regex to match and remove "ttl": "xxx" from cache_control objects
// Matches: "ttl": "5m", "ttl": "3600", etc. (with optional trailing comma)
// See: ampAnthropicMessage() 函数中的 TTL 过滤逻辑
var cacheControlTTLRegex = regexp.MustCompile(`"ttl"\s*:\s*"[^"]*"\s*,?\s*`)

var (
	ampProxy     *httputil.ReverseProxy
	ampProxyOnce sync.Once
	ampProxyMu   sync.RWMutex
)

func init() {
	// Provider Aliases - 复用 octopus relay 系统
	// OpenAI 兼容路由
	router.NewGroupRouter("/api/provider/openai/v1").
		Use(middleware.APIKeyAuth()).
		Use(middleware.RequireJSON()).
		AddRoute(
			router.NewRoute("/chat/completions", http.MethodPost).
				Handle(ampOpenAIChat),
		).
		AddRoute(
			router.NewRoute("/responses", http.MethodPost).
				Handle(ampOpenAIResponse),
		)

	// Anthropic 兼容路由
	router.NewGroupRouter("/api/provider/anthropic/v1").
		Use(middleware.APIKeyAuth()).
		Use(middleware.RequireJSON()).
		AddRoute(
			router.NewRoute("/messages", http.MethodPost).
				Handle(ampAnthropicMessage),
		)

	// 通用 provider 路由 (支持 :provider 参数)
	router.NewGroupRouter("/api/provider/:provider/v1").
		Use(middleware.APIKeyAuth()).
		Use(middleware.RequireJSON()).
		AddRoute(
			router.NewRoute("/chat/completions", http.MethodPost).
				Handle(ampOpenAIChat),
		).
		AddRoute(
			router.NewRoute("/responses", http.MethodPost).
				Handle(ampOpenAIResponse),
		).
		AddRoute(
			router.NewRoute("/messages", http.MethodPost).
				Handle(ampAnthropicMessage),
		)
}

// RegisterAmpManagementRoutes 注册 amp management 路由到给定的 engine
// 这个函数应该在 server.Start() 中调用
func RegisterAmpManagementRoutes(engine *gin.Engine) {
	cfg := conf.AppConfig.AmpCode
	if !cfg.Enabled {
		log.Infof("amp integration disabled")
		return
	}

	if cfg.UpstreamURL == "" {
		log.Warnf("amp upstream URL not configured, management routes disabled")
		return
	}

	// 初始化 proxy
	initAmpProxy(cfg.UpstreamURL)

	// Management 路由组
	api := engine.Group("/api")
	api.Use(ampManagementMiddleware(&cfg))

	// 代理 handler
	proxyHandler := func(c *gin.Context) {
		proxy := getAmpProxy()
		if proxy == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "amp upstream proxy not available"})
			return
		}
		proxy.ServeHTTP(c.Writer, c.Request)
	}

	// Management routes - 代理到 ampcode.com
	api.Any("/internal", proxyHandler)
	api.Any("/internal/*path", proxyHandler)
	api.Any("/user", proxyHandler)
	api.Any("/user/*path", proxyHandler)
	api.Any("/auth", proxyHandler)
	api.Any("/auth/*path", proxyHandler)
	api.Any("/meta", proxyHandler)
	api.Any("/meta/*path", proxyHandler)
	api.Any("/threads", proxyHandler)
	api.Any("/threads/*path", proxyHandler)
	api.Any("/telemetry", proxyHandler)
	api.Any("/telemetry/*path", proxyHandler)
	api.Any("/otel", proxyHandler)
	api.Any("/otel/*path", proxyHandler)
	api.Any("/tab", proxyHandler)
	api.Any("/tab/*path", proxyHandler)

	// Root-level routes
	engine.Any("/threads", ampManagementMiddleware(&cfg), proxyHandler)
	engine.Any("/threads/*path", ampManagementMiddleware(&cfg), proxyHandler)
	engine.Any("/auth", ampManagementMiddleware(&cfg), proxyHandler)
	engine.Any("/auth/*path", ampManagementMiddleware(&cfg), proxyHandler)
	engine.Any("/docs", ampManagementMiddleware(&cfg), proxyHandler)
	engine.Any("/docs/*path", ampManagementMiddleware(&cfg), proxyHandler)
	engine.Any("/settings", ampManagementMiddleware(&cfg), proxyHandler)
	engine.Any("/settings/*path", ampManagementMiddleware(&cfg), proxyHandler)

	log.Infof("amp management routes registered, upstream: %s", cfg.UpstreamURL)
}

// ampManagementMiddleware 返回 management 路由的中间件
func ampManagementMiddleware(cfg *conf.AmpCode) gin.HandlerFunc {
	return func(c *gin.Context) {
		// localhost 限制检查
		if cfg.RestrictManagementToLocalhost {
			remoteAddr := c.Request.RemoteAddr
			host, _, err := net.SplitHostPort(remoteAddr)
			if err != nil {
				host = remoteAddr
			}
			ip := net.ParseIP(host)
			if ip == nil || !ip.IsLoopback() {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"error": "Access denied: management routes restricted to localhost",
				})
				return
			}
		}
		c.Next()
	}
}

// Provider handlers - 转发到 octopus relay
func ampOpenAIChat(c *gin.Context) {
	relay.Handler(inbound.InboundTypeOpenAIChat, c)
}

func ampOpenAIResponse(c *gin.Context) {
	relay.Handler(inbound.InboundTypeOpenAIResponse, c)
}

func ampAnthropicMessage(c *gin.Context) {
	// 读取请求体
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Warnf("amp: could not read request body: %v", err)
		relay.Handler(inbound.InboundTypeAnthropic, c)
		return
	}

	// TODO(bedrock-ttl): 当 Bedrock 支持 cache_control.ttl 后，删除以下过滤逻辑（第 197-205 行）
	// 过滤掉 cache_control 中的 ttl 字段（Bedrock 不支持）
	if cacheControlTTLRegex.Match(bodyBytes) {
		bodyBytes = cacheControlTTLRegex.ReplaceAll(bodyBytes, []byte(""))
		// 清理可能留下的空 cache_control 对象中的尾随逗号
		// {"type": "ephemeral", } -> {"type": "ephemeral"}
		bodyBytes = bytes.ReplaceAll(bodyBytes, []byte(`, }`), []byte(`}`))
		bodyBytes = bytes.ReplaceAll(bodyBytes, []byte(`,}`), []byte(`}`))
		log.Infof("amp: filtered cache_control.ttl from request for Bedrock compatibility")
	}
	// END TODO(bedrock-ttl)

	// 重新设置请求体
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	c.Request.ContentLength = int64(len(bodyBytes))
	c.Request.Header.Set("Content-Length", strconv.Itoa(len(bodyBytes)))

	relay.Handler(inbound.InboundTypeAnthropic, c)
}

// Proxy 相关函数

func initAmpProxy(upstreamURL string) {
	ampProxyOnce.Do(func() {
		proxy, err := createAmpReverseProxy(upstreamURL)
		if err != nil {
			log.Errorf("failed to create amp proxy: %v", err)
			return
		}
		ampProxyMu.Lock()
		ampProxy = proxy
		ampProxyMu.Unlock()
		log.Infof("amp reverse proxy initialized for: %s", upstreamURL)
	})
}

func getAmpProxy() *httputil.ReverseProxy {
	ampProxyMu.RLock()
	defer ampProxyMu.RUnlock()
	return ampProxy
}

// createAmpReverseProxy 创建到 ampcode.com 的反向代理
func createAmpReverseProxy(upstreamURL string) (*httputil.ReverseProxy, error) {
	parsed, err := url.Parse(upstreamURL)
	if err != nil {
		return nil, fmt.Errorf("invalid amp upstream url: %w", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(parsed)
	originalDirector := proxy.Director

	// 获取 secret source
	secretSource := NewMultiSourceSecret(conf.AppConfig.AmpCode.UpstreamAPIKey)

	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = parsed.Host

		// 移除客户端的 Authorization header
		req.Header.Del("Authorization")
		req.Header.Del("X-Api-Key")

		// 注入 API key
		if key, err := secretSource.Get(req.Context()); err == nil && key != "" {
			req.Header.Set("X-Api-Key", key)
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", key))
		}

		// 拦截 webSearch2 和 extractWebPageContent 请求，强制使用免费层
		query := req.URL.RawQuery
		path := req.URL.Path
		if path == internalAPIPath && (query == webSearchQuery || query == extractWebPageContentQuery) && req.Body != nil {
			bodyBytes, err := io.ReadAll(req.Body)
			if err != nil {
				log.Warnf("could not read request body for %s: %v", query, err)
				req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			} else if isFreeTierRequestRegex.Match(bodyBytes) {
				// 将 isFreeTierRequest: false 替换为 true
				modifiedBody := isFreeTierRequestRegex.ReplaceAll(bodyBytes, []byte(`"isFreeTierRequest":true`))
				req.ContentLength = int64(len(modifiedBody))
				req.Header.Set("Content-Length", strconv.Itoa(len(modifiedBody)))
				req.Body = io.NopCloser(bytes.NewBuffer(modifiedBody))
				log.Infof("Modified %s request to use free tier", query)
			} else {
				req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			}
		}
	}

	// 处理 gzip 压缩响应
	proxy.ModifyResponse = func(resp *http.Response) error {
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil
		}

		if resp.Header.Get("Content-Encoding") != "" {
			return nil
		}

		// 检测 gzip magic bytes
		originalBody := resp.Body
		header := make([]byte, 2)
		n, _ := io.ReadFull(originalBody, header)

		if n >= 2 && header[0] == 0x1f && header[1] == 0x8b {
			rest, err := io.ReadAll(originalBody)
			if err != nil {
				resp.Body = &readCloser{
					r: io.MultiReader(bytes.NewReader(header[:n]), originalBody),
					c: originalBody,
				}
				return nil
			}

			gzippedData := append(header[:n], rest...)
			gzipReader, err := gzip.NewReader(bytes.NewReader(gzippedData))
			if err != nil {
				_ = originalBody.Close()
				resp.Body = io.NopCloser(bytes.NewReader(gzippedData))
				return nil
			}

			decompressed, err := io.ReadAll(gzipReader)
			_ = gzipReader.Close()
			if err != nil {
				_ = originalBody.Close()
				resp.Body = io.NopCloser(bytes.NewReader(gzippedData))
				return nil
			}

			_ = originalBody.Close()
			resp.Body = io.NopCloser(bytes.NewReader(decompressed))
			resp.ContentLength = int64(len(decompressed))
			resp.Header.Del("Content-Encoding")
			resp.Header.Set("Content-Length", strconv.FormatInt(resp.ContentLength, 10))
		} else {
			resp.Body = &readCloser{
				r: io.MultiReader(bytes.NewReader(header[:n]), originalBody),
				c: originalBody,
			}
		}

		return nil
	}

	proxy.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, err error) {
		log.Errorf("amp upstream proxy error for %s %s: %v", req.Method, req.URL.Path, err)
		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusBadGateway)
		_, _ = rw.Write([]byte(`{"error":"amp_upstream_proxy_error","message":"Failed to reach Amp upstream"}`))
	}

	return proxy, nil
}

// readCloser wraps a reader and forwards Close to a separate closer.
type readCloser struct {
	r io.Reader
	c io.Closer
}

func (rc *readCloser) Read(p []byte) (int, error) { return rc.r.Read(p) }
func (rc *readCloser) Close() error               { return rc.c.Close() }

// MultiSourceSecret 多源密钥管理
type MultiSourceSecret struct {
	explicitKey string
	envKey      string
	filePath    string
	cacheTTL    time.Duration
	mu          sync.RWMutex
	cache       *cachedSecret
}

type cachedSecret struct {
	value     string
	expiresAt time.Time
}

func NewMultiSourceSecret(explicitKey string) *MultiSourceSecret {
	home, _ := os.UserHomeDir()
	filePath := filepath.Join(home, ".local", "share", "amp", "secrets.json")

	return &MultiSourceSecret{
		explicitKey: strings.TrimSpace(explicitKey),
		envKey:      "AMP_API_KEY",
		filePath:    filePath,
		cacheTTL:    5 * time.Minute,
	}
}

func (s *MultiSourceSecret) Get(ctx context.Context) (string, error) {
	// 优先级 1: 配置文件
	if s.explicitKey != "" {
		return s.explicitKey, nil
	}

	// 优先级 2: 环境变量
	if envValue := strings.TrimSpace(os.Getenv(s.envKey)); envValue != "" {
		return envValue, nil
	}

	// 优先级 3: 文件
	s.mu.RLock()
	if s.cache != nil && time.Now().Before(s.cache.expiresAt) {
		value := s.cache.value
		s.mu.RUnlock()
		return value, nil
	}
	s.mu.RUnlock()

	key, err := s.readFromFile()
	if err != nil {
		s.updateCache("")
		return "", err
	}

	s.updateCache(key)
	return key, nil
}

func (s *MultiSourceSecret) readFromFile() (string, error) {
	content, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("failed to read amp secrets from %s: %w", s.filePath, err)
	}

	var secrets map[string]string
	if err := json.Unmarshal(content, &secrets); err != nil {
		return "", fmt.Errorf("failed to parse amp secrets from %s: %w", s.filePath, err)
	}

	key := strings.TrimSpace(secrets["apiKey@https://ampcode.com/"])
	return key, nil
}

func (s *MultiSourceSecret) updateCache(value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache = &cachedSecret{
		value:     value,
		expiresAt: time.Now().Add(s.cacheTTL),
	}
}
