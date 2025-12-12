package inbound

import (
	"github.com/bestruirui/octopus/internal/transformer/inbound/anthropic"
	"github.com/bestruirui/octopus/internal/transformer/inbound/openai"
	"github.com/bestruirui/octopus/internal/transformer/model"
)

type InboundType int

const (
	InboundTypeOpenAI InboundType = iota
	InboundTypeOpenAIChat
	InboundTypeOpenAIResponse
	InboundTypeAnthropic
)

var inboundFactories = map[InboundType]func() model.Inbound{
	InboundTypeOpenAIChat: func() model.Inbound { return &openai.ChatInbound{} },
	InboundTypeAnthropic:  func() model.Inbound { return &anthropic.MessagesInbound{} },
}

func Get(inboundType InboundType) model.Inbound {
	if factory, ok := inboundFactories[inboundType]; ok {
		return factory()
	}
	return nil
}
