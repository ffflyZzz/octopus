package outbound

import (
	"octopus/internal/transformer/model"
	"octopus/internal/transformer/outbound/antigravity"
	"octopus/internal/transformer/outbound/authropic"
	"octopus/internal/transformer/outbound/gemini"
	"octopus/internal/transformer/outbound/openai"
)

type OutboundType int

const (
	OutboundTypeOpenAIChat OutboundType = iota
	OutboundTypeOpenAIResponse
	OutboundTypeAnthropic
	OutboundTypeGemini
	OutboundTypeAntigravity
)

var outboundFactories = map[OutboundType]func() model.Outbound{
	OutboundTypeOpenAIChat:     func() model.Outbound { return &openai.ChatOutbound{} },
	OutboundTypeOpenAIResponse: func() model.Outbound { return &openai.ResponseOutbound{} },
	OutboundTypeAnthropic:      func() model.Outbound { return &authropic.MessageOutbound{} },
	OutboundTypeGemini:         func() model.Outbound { return &gemini.MessagesOutbound{} },
	OutboundTypeAntigravity:    func() model.Outbound { return &antigravity.MessageOutbound{} },
}

func Get(outboundType OutboundType) model.Outbound {
	if factory, ok := outboundFactories[outboundType]; ok {
		return factory()
	}
	return nil
}
