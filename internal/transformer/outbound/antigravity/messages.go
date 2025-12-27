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
	"sync/atomic"
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

	// 工具调用签名 - 用于绕过签名验证
	functionThoughtSignature = "skip_thought_signature_validator"
)

// functionCallIDCounter 用于生成唯一的工具调用 ID
var functionCallIDCounter uint64

// tokenCache 用于缓存 access token
type tokenCache struct {
	mu         sync.RWMutex
	tokens     map[string]*cachedToken
	httpClient *http.Client
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
type MessageOutbound struct {
	toolIndex int // 用于流式响应中跟踪工具调用索引
}

// AntigravityRequest Antigravity API 请求格式
type AntigravityRequest struct {
	Model   string                  `json:"model"`
	Request AntigravityInnerRequest `json:"request"`
}

// AntigravityInnerRequest Antigravity API 内部请求格式
type AntigravityInnerRequest struct {
	Contents          []AntigravityContent     `json:"contents"`
	GenerationConfig  *GenerationConfig        `json:"generationConfig,omitempty"`
	SafetySettings    []map[string]interface{} `json:"safetySettings,omitempty"`
	SystemInstruction *AntigravityContent      `json:"systemInstruction,omitempty"`
	Tools             []AntigravityTool        `json:"tools,omitempty"`
}

// AntigravityContent Antigravity content 格式
type AntigravityContent struct {
	Role  string                   `json:"role"`
	Parts []map[string]interface{} `json:"parts"`
}

// AntigravityTool 工具定义
type AntigravityTool struct {
	FunctionDeclarations []FunctionDeclaration  `json:"functionDeclarations,omitempty"`
	GoogleSearch         map[string]interface{} `json:"googleSearch,omitempty"`
}

// FunctionDeclaration 函数声明
type FunctionDeclaration struct {
	Name                 string                 `json:"name"`
	Description          string                 `json:"description,omitempty"`
	ParametersJsonSchema map[string]interface{} `json:"parametersJsonSchema,omitempty"`
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
			ThoughtsTokenCount   int `json:"thoughtsTokenCount,omitempty"`
		} `json:"usageMetadata,omitempty"`
		ModelVersion string `json:"modelVersion,omitempty"`
		ResponseID   string `json:"responseId,omitempty"`
		CreateTime   string `json:"createTime,omitempty"`
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

	// 提取 system message 和非 system messages
	var systemInstruction *AntigravityContent
	var nonSystemMessages []model.Message

	for _, msg := range request.Messages {
		if msg.Role == "system" {
			// 将 system message 转换为 systemInstruction
			var parts []map[string]interface{}
			if msg.Content.Content != nil && *msg.Content.Content != "" {
				parts = append(parts, map[string]interface{}{"text": *msg.Content.Content})
			} else if len(msg.Content.MultipleContent) > 0 {
				for _, part := range msg.Content.MultipleContent {
					if part.Type == "text" && part.Text != nil {
						parts = append(parts, map[string]interface{}{"text": *part.Text})
					}
				}
			}
			if len(parts) > 0 {
				systemInstruction = &AntigravityContent{
					Role:  "user",
					Parts: parts,
				}
			}
		} else {
			nonSystemMessages = append(nonSystemMessages, msg)
		}
	}

	// 转换 messages 为 Antigravity contents 格式
	contents, err := convertMessagesToContents(nonSystemMessages)
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

	// 转换工具定义
	var tools []AntigravityTool
	if len(request.Tools) > 0 {
		var functionDeclarations []FunctionDeclaration
		for _, tool := range request.Tools {
			if tool.Type == "function" {
				// 解析 parameters JSON
				var parametersSchema map[string]interface{}
				if len(tool.Function.Parameters) > 0 {
					if err := json.Unmarshal(tool.Function.Parameters, &parametersSchema); err != nil {
						// 如果解析失败，使用默认的空 schema
						parametersSchema = map[string]interface{}{
							"type":       "object",
							"properties": map[string]interface{}{},
						}
					}
				} else {
					parametersSchema = map[string]interface{}{
						"type":       "object",
						"properties": map[string]interface{}{},
					}
				}

				functionDeclarations = append(functionDeclarations, FunctionDeclaration{
					Name:                 tool.Function.Name,
					Description:          tool.Function.Description,
					ParametersJsonSchema: parametersSchema,
				})
			}
		}

		if len(functionDeclarations) > 0 {
			tools = append(tools, AntigravityTool{
				FunctionDeclarations: functionDeclarations,
			})
		}
	}

	// 构建 Antigravity 请求
	antigravityReq := AntigravityRequest{
		Model: request.Model,
		Request: AntigravityInnerRequest{
			Contents:          contents,
			GenerationConfig:  generationConfig,
			SystemInstruction: systemInstruction,
			Tools:             tools,
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

	// 始终使用 streamGenerateContent 端点
	// 注意：generateContent 端点可能需要不同的权限（Cloud Project License）
	// 而 streamGenerateContent 端点可以正常工作
	// 同时始终添加 ?alt=sse 参数
	endpoint := antigravityStreamPath  // 始终使用流式端点
	query := "?alt=sse"  // 始终添加 alt=sse 参数
	isStream := request.Stream != nil && *request.Stream

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

	// 由于使用 streamGenerateContent?alt=sse 端点，响应是 SSE 格式
	// 需要解析 SSE 数据，提取所有 data: 行的内容
	var content string
	var finishReason *string
	var usage *model.Usage
	var traceID string
	var toolCalls []model.ToolCall
	var hasFunctionCall bool
	var reasoningContent string

	lines := strings.Split(string(body), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		jsonData := strings.TrimPrefix(line, "data: ")
		if jsonData == "" || jsonData == "[DONE]" {
			continue
		}

		var antigravityResp AntigravityResponse
		if err := json.Unmarshal([]byte(jsonData), &antigravityResp); err != nil {
			// 忽略解析错误，可能是部分数据
			continue
		}

		// 提取 traceID
		if antigravityResp.TraceID != "" {
			traceID = antigravityResp.TraceID
		}

		// 提取内容
		if len(antigravityResp.Response.Candidates) > 0 {
			candidate := antigravityResp.Response.Candidates[0]
			if candidate.FinishReason != "" {
				fr := strings.ToLower(candidate.FinishReason)
				finishReason = &fr
			}

			for _, part := range candidate.Content.Parts {
				// 处理文本内容
				if text, ok := part["text"].(string); ok {
					// 检查是否是思考内容
					if thought, ok := part["thought"].(bool); ok && thought {
						reasoningContent += text
					} else {
						content += text
					}
				}

				// 处理 functionCall
				if fc, ok := part["functionCall"].(map[string]interface{}); ok {
					hasFunctionCall = true
					name, _ := fc["name"].(string)
					args, _ := fc["args"].(map[string]interface{})

					argsJSON, _ := json.Marshal(args)

					toolCall := model.ToolCall{
						ID:    generateFunctionCallID(name),
						Type:  "function",
						Index: len(toolCalls),
						Function: model.FunctionCall{
							Name:      name,
							Arguments: string(argsJSON),
						},
					}
					toolCalls = append(toolCalls, toolCall)
				}

				// 处理 inlineData (图片输出)
				if inlineData, ok := part["inlineData"].(map[string]interface{}); ok {
					mimeType, _ := inlineData["mime_type"].(string)
					if mimeType == "" {
						mimeType, _ = inlineData["mimeType"].(string)
					}
					data, _ := inlineData["data"].(string)
					if mimeType != "" && data != "" {
						// 将图片添加为 base64 URL 格式的内容
						imageURL := fmt.Sprintf("data:%s;base64,%s", mimeType, data)
						if content != "" {
							content += "\n"
						}
						content += fmt.Sprintf("![image](%s)", imageURL)
					}
				}
			}
		}

		// 提取 token 使用信息（通常在最后一个 chunk）
		if antigravityResp.Response.UsageMetadata != nil {
			usage = &model.Usage{
				PromptTokens:     int64(antigravityResp.Response.UsageMetadata.PromptTokenCount),
				CompletionTokens: int64(antigravityResp.Response.UsageMetadata.CandidatesTokenCount),
				TotalTokens:      int64(antigravityResp.Response.UsageMetadata.TotalTokenCount),
			}
		}
	}

	// 如果有 functionCall，设置 finish_reason 为 tool_calls
	if hasFunctionCall && (finishReason == nil || *finishReason == "stop") {
		fr := "tool_calls"
		finishReason = &fr
	}

	// 构建响应消息
	message := &model.Message{
		Role: "assistant",
		Content: model.MessageContent{
			Content: &content,
		},
		ToolCalls: toolCalls,
	}

	// 添加 reasoning content
	if reasoningContent != "" {
		message.ReasoningContent = &reasoningContent
	}

	// 构建标准的 InternalLLMResponse
	return &model.InternalLLMResponse{
		ID:      traceID,
		Object:  "chat.completion",
		Created: 0, // Antigravity 不提供时间戳
		Model:   "", // 从请求中获取
		Choices: []model.Choice{
			{
				Index:        0,
				Message:      message,
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
	var toolCalls []model.ToolCall
	var hasFunctionCall bool
	var reasoningContent string

	if len(antigravityResp.Response.Candidates) > 0 {
		candidate := antigravityResp.Response.Candidates[0]
		if candidate.FinishReason != "" {
			fr := strings.ToLower(candidate.FinishReason)
			finishReason = &fr
		}

		for _, part := range candidate.Content.Parts {
			// 处理文本内容
			if text, ok := part["text"].(string); ok {
				// 检查是否是思考内容
				if thought, ok := part["thought"].(bool); ok && thought {
					reasoningContent += text
				} else {
					content += text
				}
			}

			// 处理 functionCall
			if fc, ok := part["functionCall"].(map[string]interface{}); ok {
				hasFunctionCall = true
				name, _ := fc["name"].(string)
				args, _ := fc["args"].(map[string]interface{})

				argsJSON, _ := json.Marshal(args)

				toolCall := model.ToolCall{
					ID:    generateFunctionCallID(name),
					Type:  "function",
					Index: m.toolIndex,
					Function: model.FunctionCall{
						Name:      name,
						Arguments: string(argsJSON),
					},
				}
				toolCalls = append(toolCalls, toolCall)
				m.toolIndex++
			}

			// 处理 inlineData (图片输出)
			if inlineData, ok := part["inlineData"].(map[string]interface{}); ok {
				mimeType, _ := inlineData["mime_type"].(string)
				if mimeType == "" {
					mimeType, _ = inlineData["mimeType"].(string)
				}
				data, _ := inlineData["data"].(string)
				if mimeType != "" && data != "" {
					imageURL := fmt.Sprintf("data:%s;base64,%s", mimeType, data)
					if content != "" {
						content += "\n"
					}
					content += fmt.Sprintf("![image](%s)", imageURL)
				}
			}
		}
	}

	// 如果有 functionCall，设置 finish_reason 为 tool_calls
	if hasFunctionCall && (finishReason == nil || *finishReason == "stop") {
		fr := "tool_calls"
		finishReason = &fr
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

	// 构建流式响应消息
	delta := &model.Message{
		Role: "assistant",
		Content: model.MessageContent{
			Content: &content,
		},
		ToolCalls: toolCalls,
	}

	// 添加 reasoning content
	if reasoningContent != "" {
		delta.ReasoningContent = &reasoningContent
	}

	// 构建流式响应
	return &model.InternalLLMResponse{
		ID:      antigravityResp.TraceID,
		Object:  "chat.completion.chunk",
		Created: 0,
		Model:   "",
		Choices: []model.Choice{
			{
				Index:        0,
				Delta:        delta,
				FinishReason: finishReason,
			},
		},
		Usage: usage,
	}, nil
}

// convertMessagesToContents 将标准 messages 格式转换为 Antigravity contents 格式
// 支持: 文本、图片(image_url)、工具调用(tool_calls)、工具响应(tool role)
func convertMessagesToContents(messages []model.Message) ([]AntigravityContent, error) {
	var contents []AntigravityContent

	// 第一遍: 构建 tool_call_id -> function_name 的映射
	toolCallIDToName := make(map[string]string)
	for _, msg := range messages {
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			for _, tc := range msg.ToolCalls {
				if tc.ID != "" && tc.Function.Name != "" {
					toolCallIDToName[tc.ID] = tc.Function.Name
				}
			}
		}
	}

	// 第二遍: 构建 tool_call_id -> response 的映射
	toolResponses := make(map[string]string)
	for _, msg := range messages {
		if msg.Role == "tool" && msg.ToolCallID != nil {
			content := ""
			if msg.Content.Content != nil {
				content = *msg.Content.Content
			}
			toolResponses[*msg.ToolCallID] = content
		}
	}

	for _, msg := range messages {
		// 跳过 tool role 消息，它们会在 assistant 消息处理时一起处理
		if msg.Role == "tool" {
			continue
		}

		// 映射角色
		role := msg.Role
		if role == "assistant" {
			role = "model"
		}

		// 构建 parts
		var parts []map[string]interface{}

		// 处理文本内容
		if msg.Content.Content != nil && *msg.Content.Content != "" {
			parts = append(parts, map[string]interface{}{
				"text": *msg.Content.Content,
			})
		}

		// 处理多模态内容
		if len(msg.Content.MultipleContent) > 0 {
			for _, part := range msg.Content.MultipleContent {
				switch part.Type {
				case "text":
					if part.Text != nil && *part.Text != "" {
						parts = append(parts, map[string]interface{}{
							"text": *part.Text,
						})
					}
				case "image_url":
					if part.ImageURL != nil && part.ImageURL.URL != "" {
						// 解析 data:image/...;base64,... 格式
						imageURL := part.ImageURL.URL
						if strings.HasPrefix(imageURL, "data:") {
							// 格式: data:image/png;base64,xxxx
							urlParts := strings.SplitN(imageURL[5:], ";", 2)
							if len(urlParts) == 2 && strings.HasPrefix(urlParts[1], "base64,") {
								mimeType := urlParts[0]
								data := urlParts[1][7:] // 去掉 "base64,"
								parts = append(parts, map[string]interface{}{
									"inlineData": map[string]interface{}{
										"mime_type": mimeType,
										"data":      data,
									},
								})
							}
						}
					}
				}
			}
		}

		// 处理 assistant 消息中的 tool_calls
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			var functionCallIDs []string

			for _, tc := range msg.ToolCalls {
				if tc.Type != "function" {
					continue
				}

				// 解析 arguments
				var args map[string]interface{}
				if tc.Function.Arguments != "" {
					if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
						// 如果解析失败，将 arguments 作为字符串放入
						args = map[string]interface{}{"params": tc.Function.Arguments}
					}
				} else {
					args = map[string]interface{}{}
				}

				parts = append(parts, map[string]interface{}{
					"functionCall": map[string]interface{}{
						"name": tc.Function.Name,
						"args": args,
					},
					"thoughtSignature": functionThoughtSignature,
				})

				if tc.ID != "" {
					functionCallIDs = append(functionCallIDs, tc.ID)
				}
			}

			// 添加当前 assistant 消息的 content
			if len(parts) > 0 {
				contents = append(contents, AntigravityContent{
					Role:  role,
					Parts: parts,
				})
			}

			// 查找后续的 tool 响应消息，构建 functionResponse
			var toolParts []map[string]interface{}
			for _, tcID := range functionCallIDs {
				if name, ok := toolCallIDToName[tcID]; ok {
					resp := toolResponses[tcID]
					if resp == "" {
						resp = "{}"
					}

					// 尝试解析为 JSON
					var responseResult interface{}
					if err := json.Unmarshal([]byte(resp), &responseResult); err != nil {
						// 如果不是有效 JSON，作为字符串处理
						responseResult = resp
					}

					toolParts = append(toolParts, map[string]interface{}{
						"functionResponse": map[string]interface{}{
							"name": name,
							"response": map[string]interface{}{
								"result": responseResult,
							},
						},
					})
				}
			}

			// 添加 tool 响应作为 user 消息
			if len(toolParts) > 0 {
				contents = append(contents, AntigravityContent{
					Role:  "user",
					Parts: toolParts,
				})
			}

			continue // 已经处理完这个 assistant 消息
		}

		// 添加普通消息
		if len(parts) > 0 {
			contents = append(contents, AntigravityContent{
				Role:  role,
				Parts: parts,
			})
		}
	}

	return contents, nil
}

// generateFunctionCallID 生成唯一的工具调用 ID
func generateFunctionCallID(name string) string {
	counter := atomic.AddUint64(&functionCallIDCounter, 1)
	return fmt.Sprintf("%s-%d-%d", name, time.Now().UnixNano(), counter)
}
