package antigravity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"octopus/internal/transformer/model"
)

const (
	antigravityBaseURL          = "https://daily-cloudcode-pa.sandbox.googleapis.com"
	antigravityStreamPath       = "/v1internal:streamGenerateContent"
	antigravityGeneratePath     = "/v1internal:generateContent"
	antigravityDefaultUserAgent = "antigravity/1.11.3 windows/amd64"
	antigravityClientID         = "1071006060591-tmhssin2h21lcre235vtolojh4g403ep.apps.googleusercontent.com"
	antigravityClientSecret     = "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"
	tokenRefreshThreshold       = 5 * time.Minute // 提前5分钟刷新
)

// tokenCache 用于缓存 access token
type tokenCache struct {
	mu          sync.RWMutex
	tokens      map[string]*cachedToken
	httpClient  *http.Client
}

type cachedToken struct {
	accessToken  string
	refreshToken string
	expiresAt    time.Time
}

var globalTokenCache = &tokenCache{
	tokens:     make(map[string]*cachedToken),
	httpClient: &http.Client{Timeout: 30 * time.Second},
}

// MessageOutbound 处理 Antigravity API 的请求和响应转换
type MessageOutbound struct{}

// AntigravityRequest Antigravity API 请求格式
type AntigravityRequest struct {
	Model   string                 `json:"model"`
	Request AntigravityInnerRequest `json:"request"`
}

// AntigravityInnerRequest Antigravity API 内部请求格式
type AntigravityInnerRequest struct {
	Contents         []AntigravityContent    `json:"contents"`
	GenerationConfig *GenerationConfig       `json:"generationConfig,omitempty"`
	SafetySettings   []map[string]interface{} `json:"safetySettings,omitempty"`
}

// AntigravityContent Antigravity content 格式
type AntigravityContent struct {
	Role  string                   `json:"role"`
	Parts []map[string]interface{} `json:"parts"`
}

// GenerationConfig Gemini generation 配置
type GenerationConfig struct {
	Temperature     *float64 `json:"temperature,omitempty"`
	TopP            *float64 `json:"topP,omitempty"`
	TopK            *int     `json:"topK,omitempty"`
	MaxOutputTokens *int     `json:"maxOutputTokens,omitempty"`
}

// AntigravityResponse Antigravity API 响应格式
type AntigravityResponse struct {
	Response struct {
		Candidates []struct {
			Content struct {
				Role  string                   `json:"role"`
				Parts []map[string]interface{} `json:"parts"`
			} `json:"content"`
			FinishReason string `json:"finishReason"`
		} `json:"candidates"`
		UsageMetadata *struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
			TotalTokenCount      int `json:"totalTokenCount"`
		} `json:"usageMetadata,omitempty"`
	} `json:"response"`
	TraceID string `json:"traceId,omitempty"`
}

// TransformRequest 将内部请求转换为 Antigravity API 请求格式
func (m *MessageOutbound) TransformRequest(ctx context.Context, request *model.InternalLLMRequest, baseURL, key string) (*http.Request, error) {
	// 获取或刷新 access token
	accessToken, err := getAccessToken(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get access token: %w", err)
	}

	// 转换 messages 为 Antigravity contents 格式
	contents, err := convertMessagesToContents(request.Messages)
	if err != nil {
		return nil, fmt.Errorf("failed to convert messages: %w", err)
	}

	// 构建 generation config
	var generationConfig *GenerationConfig
	if request.MaxTokens != nil || request.Temperature != nil || request.TopP != nil {
		generationConfig = &GenerationConfig{
			Temperature: request.Temperature,
			TopP:        request.TopP,
		}
		// 转换 MaxTokens 从 *int64 到 *int
		if request.MaxTokens != nil {
			maxTokens := int(*request.MaxTokens)
			generationConfig.MaxOutputTokens = &maxTokens
		} else {
			defaultMaxTokens := 8096
			generationConfig.MaxOutputTokens = &defaultMaxTokens
		}
	}

	// 构建 Antigravity 请求
	antigravityReq := AntigravityRequest{
		Model: request.Model,
		Request: AntigravityInnerRequest{
			Contents:         contents,
			GenerationConfig: generationConfig,
		},
	}

	body, err := json.Marshal(antigravityReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// 确定请求 URL
	if baseURL == "" {
		baseURL = antigravityBaseURL
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	endpoint := antigravityGeneratePath
	query := ""
	isStream := request.Stream != nil && *request.Stream
	if isStream {
		endpoint = antigravityStreamPath
		query = "?alt=sse"
	}

	url := baseURL + endpoint + query

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// 设置请求头 - 使用刷新后的 access token
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("User-Agent", antigravityDefaultUserAgent)

	if isStream {
		req.Header.Set("Accept", "text/event-stream")
	} else {
		req.Header.Set("Accept", "application/json")
	}

	return req, nil
}

// getAccessToken 获取或刷新 access token
// key 可以是 refresh_token (以 1// 开头) 或 access_token
func getAccessToken(ctx context.Context, key string) (string, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", fmt.Errorf("empty key provided")
	}

	// 如果不是 refresh token，直接返回（假设是 access token）
	if !strings.HasPrefix(key, "1//") {
		return key, nil
	}

	// 是 refresh token，需要获取/刷新 access token
	globalTokenCache.mu.Lock()
	defer globalTokenCache.mu.Unlock()

	// 检查缓存
	if cached, ok := globalTokenCache.tokens[key]; ok {
		// 如果 token 还有效（距离过期时间大于阈值）
		if time.Now().Add(tokenRefreshThreshold).Before(cached.expiresAt) {
			return cached.accessToken, nil
		}
	}

	// 刷新 token
	tokenResp, err := refreshToken(ctx, key)
	if err != nil {
		return "", fmt.Errorf("failed to refresh token: %w", err)
	}

	// 缓存新的 token
	expiresAt := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	globalTokenCache.tokens[key] = &cachedToken{
		accessToken:  tokenResp.AccessToken,
		refreshToken: key,
		expiresAt:    expiresAt,
	}

	return tokenResp.AccessToken, nil
}

// tokenResponse OAuth token 响应
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

// refreshToken 使用 refresh token 获取新的 access token
func refreshToken(ctx context.Context, refreshToken string) (*tokenResponse, error) {
	data := url.Values{}
	data.Set("client_id", antigravityClientID)
	data.Set("client_secret", antigravityClientSecret)
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", refreshToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://oauth2.googleapis.com/token",
		strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Host", "oauth2.googleapis.com")
	req.Header.Set("User-Agent", antigravityDefaultUserAgent)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := globalTokenCache.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("token refresh failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var tokenResp tokenResponse
	if err := json.Unmarshal(bodyBytes, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	return &tokenResp, nil
}

// TransformResponse 将 Antigravity 响应转换为内部格式
func (m *MessageOutbound) TransformResponse(ctx context.Context, response *http.Response) (*model.InternalLLMResponse, error) {
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if len(body) == 0 {
		return nil, fmt.Errorf("response body is empty")
	}

	var antigravityResp AntigravityResponse
	if err := json.Unmarshal(body, &antigravityResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal antigravity response: %w", err)
	}

	// 提取内容
	var content string
	var finishReason *string
	if len(antigravityResp.Response.Candidates) > 0 {
		candidate := antigravityResp.Response.Candidates[0]
		if candidate.FinishReason != "" {
			finishReason = &candidate.FinishReason
		}
		for _, part := range candidate.Content.Parts {
			if text, ok := part["text"].(string); ok {
				content += text
			}
		}
	}

	// 提取 token 使用信息
	var usage *model.Usage
	if antigravityResp.Response.UsageMetadata != nil {
		usage = &model.Usage{
			PromptTokens:     int64(antigravityResp.Response.UsageMetadata.PromptTokenCount),
			CompletionTokens: int64(antigravityResp.Response.UsageMetadata.CandidatesTokenCount),
			TotalTokens:      int64(antigravityResp.Response.UsageMetadata.TotalTokenCount),
		}
	}

	// 构建标准的 InternalLLMResponse
	return &model.InternalLLMResponse{
		ID:      antigravityResp.TraceID,
		Object:  "chat.completion",
		Created: 0, // Antigravity 不提供时间戳
		Model:   "", // 从请求中获取
		Choices: []model.Choice{
			{
				Index: 0,
				Message: &model.Message{
					Role: "assistant",
					Content: model.MessageContent{
						Content: &content,
					},
				},
				FinishReason: finishReason,
			},
		},
		Usage: usage,
	}, nil
}

// TransformStream 将 Antigravity 流式响应转换为内部格式
func (m *MessageOutbound) TransformStream(ctx context.Context, eventData []byte) (*model.InternalLLMResponse, error) {
	// 处理空数据或 [DONE] 标记
	if len(eventData) == 0 || bytes.Contains(eventData, []byte("[DONE]")) {
		return &model.InternalLLMResponse{
			Object: "[DONE]",
		}, nil
	}

	// 解析 Antigravity 流式响应
	var antigravityResp AntigravityResponse
	if err := json.Unmarshal(eventData, &antigravityResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal antigravity stream chunk: %w", err)
	}

	// 提取内容
	var content string
	var finishReason *string
	if len(antigravityResp.Response.Candidates) > 0 {
		candidate := antigravityResp.Response.Candidates[0]
		if candidate.FinishReason != "" {
			finishReason = &candidate.FinishReason
		}
		for _, part := range candidate.Content.Parts {
			if text, ok := part["text"].(string); ok {
				content += text
			}
		}
	}

	// 提取 token 使用信息（通常在最后一个 chunk）
	var usage *model.Usage
	if antigravityResp.Response.UsageMetadata != nil {
		usage = &model.Usage{
			PromptTokens:     int64(antigravityResp.Response.UsageMetadata.PromptTokenCount),
			CompletionTokens: int64(antigravityResp.Response.UsageMetadata.CandidatesTokenCount),
			TotalTokens:      int64(antigravityResp.Response.UsageMetadata.TotalTokenCount),
		}
	}

	// 构建流式响应
	return &model.InternalLLMResponse{
		ID:      antigravityResp.TraceID,
		Object:  "chat.completion.chunk",
		Created: 0,
		Model:   "",
		Choices: []model.Choice{
			{
				Index: 0,
				Delta: &model.Message{
					Role: "assistant",
					Content: model.MessageContent{
						Content: &content,
					},
				},
				FinishReason: finishReason,
			},
		},
		Usage: usage,
	}, nil
}

// convertMessagesToContents 将标准 messages 格式转换为 Antigravity contents 格式
func convertMessagesToContents(messages []model.Message) ([]AntigravityContent, error) {
	var contents []AntigravityContent

	for _, msg := range messages {
		// 映射角色
		role := msg.Role
		if role == "assistant" {
			role = "model"
		}

		// 构建 parts
		var parts []map[string]interface{}

		// 处理 Content 字段
		if msg.Content.Content != nil && *msg.Content.Content != "" {
			// 单个文本内容
			parts = append(parts, map[string]interface{}{
				"text": *msg.Content.Content,
			})
		} else if len(msg.Content.MultipleContent) > 0 {
			// 多个内容部分
			for _, part := range msg.Content.MultipleContent {
				if part.Type == "text" && part.Text != nil && *part.Text != "" {
					parts = append(parts, map[string]interface{}{
						"text": *part.Text,
					})
				}
			}
		}

		if len(parts) > 0 {
			contents = append(contents, AntigravityContent{
				Role:  role,
				Parts: parts,
			})
		}
	}

	return contents, nil
}
