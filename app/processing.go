package app

import (
	"fmt"
	"strconv"
	"time"

	"github.com/verdoga/dsl-validator/model"
)

// startProcessing добавляет ровно одну запись с новым ID, toolName,
// версией и StartedAt в UTC/RFC3339Nano; DurationMs до завершения равно nil.
// Вызывается только после успешного Prepare: версия поддерживается,
// выборка диагностик непуста и целостность модели проверена.
// Document, Lines и старые Processing не переписывает.
func startProcessing(result *model.Result, version string, startedAt time.Time) (processingID string) {
	processingID = nextProcessingID(result.Processing)
	started := startedAt.UTC().Format(time.RFC3339Nano)
	result.Processing = append(result.Processing, model.Processing{
		ID:        processingID,
		Tool:      toolName,
		Version:   &version,
		StartedAt: &started,
	})
	return processingID
}

// nextProcessingID выбирает свободный pN, N>=1, с учётом всей истории.
// ID уникален внутри данного JSON, глобальная уникальность не требуется.
func nextProcessingID(entries []model.Processing) string {
	used := make(map[string]bool, len(entries))
	for _, entry := range entries {
		used[entry.ID] = true
	}
	for number := 1; ; number++ {
		id := "p" + strconv.Itoa(number)
		if !used[id] {
			return id
		}
	}
}

// finishProcessing находит ровно текущий ID и устанавливает DurationMs
// в неотрицательных миллисекундах от startedAt до завершения конвейера.
// Чтение и закрытие JSON, подготовка по версии, контроль модели и Check
// входят в длительность;
// сохранение и консольный вывод не входят, как и в существующем парсере.
// Использует монотонную составляющую time.Time; переполнение int — ошибка.
// Возвращает ту же длительность для FileReport; отсутствие ID — ошибка.
func finishProcessing(result *model.Result, processingID string, startedAt time.Time) (duration time.Duration, err error) {
	duration = max(time.Since(startedAt), 0)
	milliseconds := duration.Milliseconds()
	if milliseconds > int64(int(^uint(0)>>1)) {
		return duration, fmt.Errorf("длительность processing %q не помещается в int", processingID)
	}
	for i := range result.Processing {
		if result.Processing[i].ID == processingID {
			value := int(milliseconds)
			result.Processing[i].DurationMs = &value
			return duration, nil
		}
	}
	return duration, fmt.Errorf("processing %q отсутствует", processingID)
}
