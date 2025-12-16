package task

import (
	"context"
	"time"

	"github.com/bestruirui/octopus/internal/op"
	"github.com/bestruirui/octopus/internal/price"
)

const taskInterval = 10 * time.Minute

func RUN() {
	for {
		price.UpdateLLMPriceTask()
		op.StatsSaveDBTask()
		op.RelayLogSaveDBTask(context.Background())
		time.Sleep(taskInterval)
	}
}
