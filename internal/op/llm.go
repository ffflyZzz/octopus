package op

import (
	"context"
	"errors"
	"fmt"

	"octopus/internal/db"
	"octopus/internal/model"
	"octopus/internal/utils/cache"

	"gorm.io/gorm"
)

var llmModelCache = cache.New[string, model.LLMPrice](16)

func LLMList(ctx context.Context) ([]model.LLMInfo, error) {
	models := []model.LLMInfo{}
	if err := db.GetDB().WithContext(ctx).Find(&models).Error; err != nil {
		return nil, err
	}
	return models, nil
}

func LLMListByChannel(ctx context.Context, channelID int) ([]model.LLMInfo, error) {
	models := []model.LLMInfo{}
	if err := db.GetDB().WithContext(ctx).Where("channel_id = ?", channelID).Find(&models).Error; err != nil {
		return nil, err
	}
	return models, nil
}

func LLMUpdate(llmInfo model.LLMInfo, ctx context.Context) error {
	// 首先检查记录是否存在
	var existing model.LLMInfo
	err := db.GetDB().WithContext(ctx).
		Where("name = ? AND channel_id = ?", llmInfo.Name, llmInfo.ChannelID).
		First(&existing).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("model not found")
		}
		return fmt.Errorf("database error: %w", err)
	}

	// 使用 map 更新以支持零值字段
	updates := map[string]interface{}{
		"input":       llmInfo.Input,
		"output":      llmInfo.Output,
		"cache_read":  llmInfo.CacheRead,
		"cache_write": llmInfo.CacheWrite,
	}

	result := db.GetDB().WithContext(ctx).
		Model(&model.LLMInfo{}).
		Where("name = ? AND channel_id = ?", llmInfo.Name, llmInfo.ChannelID).
		Updates(updates)

	if result.Error != nil {
		return fmt.Errorf("update failed: %w", result.Error)
	}

	cacheKey := fmt.Sprintf("%s:%d", llmInfo.Name, llmInfo.ChannelID)
	llmModelCache.Set(cacheKey, llmInfo.LLMPrice)
	return nil
}

func LLMDelete(modelName string, channelID int, ctx context.Context) error {
	if err := db.GetDB().WithContext(ctx).Where("name = ? AND channel_id = ?", modelName, channelID).Delete(&model.LLMInfo{}).Error; err != nil {
		return err
	}
	// 清除该模型的缓存 (格式: "modelName:channelID")
	cacheKey := fmt.Sprintf("%s:%d", modelName, channelID)
	llmModelCache.Del(cacheKey)
	return nil
}

// LLMBatchDelete 批量删除模型（上游新增功能）
func LLMBatchDelete(modelNames []string, ctx context.Context) error {
	if len(modelNames) == 0 {
		return nil
	}
	if err := db.GetDB().WithContext(ctx).Where("name IN ?", modelNames).Delete(&model.LLMInfo{}).Error; err != nil {
		return err
	}
	llmModelCache.Del(modelNames...)
	return nil
}

func LLMCreate(m model.LLMInfo, ctx context.Context) error {
	if err := db.GetDB().WithContext(ctx).Create(&m).Error; err != nil {
		return err
	}
	cacheKey := fmt.Sprintf("%s:%d", m.Name, m.ChannelID)
	llmModelCache.Set(cacheKey, m.LLMPrice)
	return nil
}

func LLMGet(name string, channelID int) (model.LLMPrice, error) {
	// 尝试从缓存获取指定渠道的价格
	cacheKey := fmt.Sprintf("%s:%d", name, channelID)
	price, ok := llmModelCache.Get(cacheKey)
	if ok {
		return price, nil
	}

	// 从数据库查询
	var m model.LLMInfo
	err := db.GetDB().Where("name = ? AND channel_id = ?", name, channelID).First(&m).Error
	if err != nil {
		// 使用 errors.Is 区分数据库错误和记录不存在
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.LLMPrice{}, fmt.Errorf("model not found")
		}
		return model.LLMPrice{}, fmt.Errorf("database error: %w", err)
	}
	llmModelCache.Set(cacheKey, m.LLMPrice)
	return m.LLMPrice, nil
}

// LLMRefreshCache 从数据库刷新 LLM 价格缓存，确保缓存与数据库完全同步
// 导出此函数以便外部调用（如定期刷新任务）
func LLMRefreshCache(ctx context.Context) error {
	models := []model.LLMInfo{}
	if err := db.GetDB().WithContext(ctx).Find(&models).Error; err != nil {
		return err
	}

	// 构建数据库中存在的键集合
	dbKeys := make(map[string]bool)
	for _, m := range models {
		cacheKey := fmt.Sprintf("%s:%d", m.Name, m.ChannelID)
		dbKeys[cacheKey] = true
		llmModelCache.Set(cacheKey, m.LLMPrice)
	}

	// 删除缓存中存在但数据库中不存在的键
	for key := range llmModelCache.GetAll() {
		if !dbKeys[key] {
			llmModelCache.Del(key)
		}
	}

	return nil
}

// llmRefreshCache 内部使用的缓存刷新函数（向后兼容）
func llmRefreshCache(ctx context.Context) error {
	return LLMRefreshCache(ctx)
}
