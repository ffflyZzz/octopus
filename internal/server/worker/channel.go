package worker

import (
	"context"
	"regexp"
	"strings"
	"time"

	"octopus/internal/model"
	"octopus/internal/op"
	"octopus/internal/price"
	"octopus/internal/utils/log"
)

func AutoGroup(channelID int, channelName, channelModel, customModel string, autoGroupType model.AutoGroupType) {
	if autoGroupType == model.AutoGroupTypeNone {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	groups, err := op.GroupList(ctx)
	if err != nil {
		log.Errorf("get group list failed: %v", err)
		return
	}

	modelNames := strings.Split(channelModel+","+customModel, ",")
	for _, modelName := range modelNames {
		// 去除首尾空格
		modelName = strings.TrimSpace(modelName)
		if modelName == "" {
			continue
		}
		for _, group := range groups {
			var matched bool
			switch autoGroupType {
			case model.AutoGroupTypeExact:
				// 准确匹配：模型名称与分组名称完全一致
				matched = strings.EqualFold(modelName, group.Name)
			case model.AutoGroupTypeFuzzy:
				// 模糊匹配：模型名称包含分组名称
				matched = strings.Contains(strings.ToLower(modelName), strings.ToLower(group.Name))
			case model.AutoGroupTypeRegex:
				if group.MatchRegex == "" {
					// 如果匹配正则为空，则使用模糊匹配
					matched = strings.EqualFold(modelName, group.Name)
					continue
				}
				// 正则匹配：模型名称与分组名称匹配
				re, err := regexp.Compile(group.MatchRegex)
				if err != nil {
					log.Warnf("compile regex failed: %v", err)
					continue
				}
				matched = re.MatchString(modelName)
			}

			if matched {
				exists := false
				for _, item := range group.Items {
					if item.ChannelID == channelID && item.ModelName == modelName {
						exists = true
						break
					}
				}
				if exists {
					break
				}
				err := op.GroupItemAdd(&model.GroupItem{
					GroupID:   group.ID,
					ChannelID: channelID,
					ModelName: modelName,
					Priority:  len(group.Items) + 1,
					Weight:    1,
				}, ctx)
				if err != nil {
					log.Errorf("add channel %s model %s to group %s failed: %v", channelName, modelName, group.Name, err)
				} else {
					log.Infof("channel %s: model [%s] added to group [%s]", channelName, modelName, group.Name)
				}
				break
			}
		}
	}
}

func CheckAndAddLLMPrice(channelID int, channelModel, customModel string) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		modelNames := strings.Split(channelModel+","+customModel, ",")
		for _, modelName := range modelNames {
			// 去除首尾空格，避免创建重复模型
			modelName = strings.TrimSpace(modelName)
			if modelName == "" {
				continue
			}
			// 检查该渠道是否已经有这个模型
			_, err := op.LLMGet(ctx, modelName, channelID)
			if err != nil {
				// 模型不存在，从价格API获取默认价格
				modelPrice := price.GetLLMPrice(modelName)
				if modelPrice == nil {
					// 如果价格API也没有，创建一个默认价格
					log.Infof("model %s price not found in API, creating with default price", modelName)
					modelPrice = &model.LLMPrice{
						Input:      0,
						Output:     0,
						CacheRead:  0,
						CacheWrite: 0,
					}
				}

				log.Infof("creating model %s for channel %d", modelName, channelID)
				err := op.LLMCreate(model.LLMInfo{
					Name:      modelName,
					ChannelID: channelID,
					LLMPrice:  *modelPrice,
				}, ctx)
				if err != nil {
					log.Errorf("failed to create model %s for channel %d: %v", modelName, channelID, err)
				}
			}
		}
	}()
}
