package authropic

import (
	"context"
	"net/http"

	"github.com/bestruirui/octopus/internal/transformer/model"
)

type MessageOutbound struct{}

func (o *MessageOutbound) TransformRequest(ctx context.Context, request *model.InternalLLMRequest, baseUrl, key string) (*http.Request, error) {

	return nil, nil
}

func (o *MessageOutbound) TransformResponse(ctx context.Context, response *http.Response) (*model.InternalLLMResponse, error) {

	return nil, nil
}

func (o *MessageOutbound) TransformStream(ctx context.Context, eventData []byte) (*model.InternalLLMResponse, error) {
	return nil, nil
}
