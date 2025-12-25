package task

import (
	"context"
	"time"

	"octopus/internal/model"
	"octopus/internal/op"
	"octopus/internal/price"
	"octopus/internal/utils/log"
)

const (
	TaskPriceUpdate    = "price_update"
	TaskStatsSave      = "stats_save"
	TaskRelayLogSave   = "relay_log_save"
	TaskSyncLLM        = "sync_llm"
	TaskLLMCacheRefresh = "llm_cache_refresh"
)

func Init() {
	priceUpdateIntervalHours, err := op.SettingGetInt(model.SettingKeyModelInfoUpdateInterval)
	if err != nil {
		log.Errorf("failed to get model info update interval: %v", err)
		return
	}
	priceUpdateInterval := time.Duration(priceUpdateIntervalHours) * time.Hour
	// 注册价格更新任务
	Register(string(model.SettingKeyModelInfoUpdateInterval), priceUpdateInterval, true, func() {
		if err := price.UpdateLLMPrice(context.Background()); err != nil {
			log.Warnf("failed to update price info: %v", err)
		}
	})

	// 注册LLM同步任务
	syncLLMIntervalHours, err := op.SettingGetInt(model.SettingKeySyncLLMInterval)
	if err != nil {
		log.Warnf("failed to get sync LLM interval: %v", err)
		return
	}
	syncLLMInterval := time.Duration(syncLLMIntervalHours) * time.Hour
	Register(string(model.SettingKeySyncLLMInterval), syncLLMInterval, true, SyncLLMTask)

	// 注册统计保存任务
	Register(TaskStatsSave, 10*time.Minute, false, op.StatsSaveDBTask)
	// 注册中继日志保存任务
	Register(TaskRelayLogSave, 10*time.Minute, false, func() {
		if err := op.RelayLogSaveDBTask(context.Background()); err != nil {
			log.Warnf("relay log save db task failed: %v", err)
		}
	})

	// 注册 LLM 价格缓存刷新任务（每5分钟刷新一次，确保缓存与数据库同步）
	Register(TaskLLMCacheRefresh, 5*time.Minute, false, func() {
		if err := op.LLMRefreshCache(context.Background()); err != nil {
			log.Warnf("llm cache refresh task failed: %v", err)
		}
	})

}
