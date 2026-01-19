package helper

import (
	"context"

	"octopus/internal/model"
	"octopus/internal/op"
	"octopus/internal/price"
)

func LLMPriceAddToDB(modelNames []string, channelID int, ctx context.Context) error {
	newLLMInfos := make([]model.LLMInfo, 0, len(modelNames))
	for _, modelName := range modelNames {
		if modelName == "" {
			continue
		}
		modelPrice := price.GetLLMPrice(modelName)
		if modelPrice != nil {
			newLLMInfos = append(newLLMInfos, model.LLMInfo{
				Name:      modelName,
				ChannelID: channelID,
				LLMPrice:  *modelPrice,
			})
		} else {
			newLLMInfos = append(newLLMInfos, model.LLMInfo{
				Name:      modelName,
				ChannelID: channelID,
			})
		}
	}
	if len(newLLMInfos) > 0 {
		// 批量创建，忽略已存在的记录
		for _, llmInfo := range newLLMInfos {
			_ = op.LLMCreate(llmInfo, ctx) // 忽略错误，因为可能已存在
		}
	}
	return nil
}

func LLMPriceDeleteFromDBWithNoPrice(modelNames []string, channelID int, ctx context.Context) error {
	if len(modelNames) == 0 {
		return nil
	}
	needDeleteModelNames := make([]string, 0, len(modelNames))
	for _, modelName := range modelNames {
		if modelName == "" {
			continue
		}
		modelPrice, err := op.LLMGet(modelName, channelID)
		if err != nil {
			// 如果没找到，跳过
			continue
		}
		if modelPrice.Input != 0 || modelPrice.Output != 0 || modelPrice.CacheRead != 0 || modelPrice.CacheWrite != 0 {
			continue
		}
		needDeleteModelNames = append(needDeleteModelNames, modelName)
	}
	// 删除指定渠道的模型
	for _, modelName := range needDeleteModelNames {
		_ = op.LLMDelete(modelName, channelID, ctx) // 忽略错误
	}
	return nil
}
