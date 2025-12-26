package model

import "octopus/internal/transformer/outbound"

// ChannelTypeInfo 渠道类型信息
type ChannelTypeInfo struct {
	Value int    `json:"value"` // 渠道类型值
	Name  string `json:"name"`  // 渠道类型名称（用于前端标识）
	Label string `json:"label"` // 渠道类型显示标签
}

// GetAllChannelTypes 获取所有渠道类型列表
func GetAllChannelTypes() []ChannelTypeInfo {
	return []ChannelTypeInfo{
		{
			Value: int(outbound.OutboundTypeOpenAIChat),
			Name:  "openai_chat",
			Label: "OpenAI Chat",
		},
		{
			Value: int(outbound.OutboundTypeOpenAIResponse),
			Name:  "openai_response",
			Label: "OpenAI Response",
		},
		{
			Value: int(outbound.OutboundTypeAnthropic),
			Name:  "anthropic",
			Label: "Anthropic",
		},
		{
			Value: int(outbound.OutboundTypeGemini),
			Name:  "gemini",
			Label: "Gemini",
		},
		{
			Value: int(outbound.OutboundTypeAntigravity),
			Name:  "antigravity",
			Label: "Antigravity",
		},
	}
}
