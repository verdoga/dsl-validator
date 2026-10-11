package app

import (
	"errors"
	"time"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/reporter"
	"github.com/verdoga/dsl-validator/storage"
	"github.com/verdoga/dsl-validator/validator"
)

// runner хранит готовые зависимости одного запуска; состояния разных файлов
// и разных Check между собой не разделяются.
type runner struct {
	// version — непустая версия исполняемого инструмента.
	version string
	// registry — подготовленный и закрытый для регистрации реестр.
	registry *diagnostics.Registry
	// reporter — единый накопитель консольной статистики.
	reporter *reporter.Reporter
}

// processFile фиксирует начало перед единственным storage.Open и вызывает
// validator.Prepare до любого изменения модели. Версия и выборка checks
// определяются заново для каждого документа, независимо от остальных файлов
// и версии исполняемого инструмента. При отказе Prepare не вызывает
// startProcessing, validator.Run или Save: даже модель в памяти не меняется.
// После успешного Prepare вызывает startProcessing и validator.Run с checks.
// После успешного Run задаёт длительность через finishProcessing
// и при err=nil сохраняет результат через storage.Save.
// При первом сбое Check, включая *validator.CheckFailure, Save не вызывает:
// текущий JSON не меняется, в том числе не дописывается Processing.
// То же правило действует при ошибке открытия, версии, целостности или конвейера.
// При отказе длительность для отчёта вычисляет без изменения Processing.
// Отказ версии и отсутствие подходящих проверок передаёт Record как error;
// сообщение в stderr содержит путь, версию и причину; ProcessingID остаётся пустым.
// При любом отказе Prepare передаёт Record с Result=nil и Updated=false.
// В любом случае Record вызывается ровно один раз с фактическим Updated.
// Возвращает причину технического отказа, сохраняя тип CheckFailure и V-код;
// ошибку вывода Record при необходимости присоединяет через errors.Join.
// Ненулевой результат запрещает Run переходить к следующему файлу.
func (r *runner) processFile(path string) error {
	startedAt := time.Now()
	result, snapshot, err := storage.Open(path)
	processingID := ""
	if err == nil {
		checks, prepareErr := validator.Prepare(result, r.registry)
		err = prepareErr
		if err != nil {
			result = nil
		} else {
			processingID = startProcessing(result, r.version, startedAt)
			err = validator.Run(result, checks, processingID)
		}
	}
	var duration time.Duration
	if err == nil {
		duration, err = finishProcessing(result, processingID, startedAt)
	} else {
		duration = max(time.Since(startedAt), 0)
	}
	updated := false
	if err == nil {
		updated, err = storage.Save(snapshot, result)
	}
	recordErr := r.reporter.Record(reporter.FileReport{
		Path: path, Result: result, ProcessingID: processingID,
		Updated: updated, Err: err, Duration: duration,
	})
	if recordErr != nil {
		return errors.Join(err, recordErr)
	}
	return err
}
