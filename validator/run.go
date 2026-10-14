package validator

import (
	"fmt"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
)

// CheckFailure — технический сбой одной проверки, а не срабатывание DSL.
// Реализует error, содержит существующий V-код сбойной диагностики
// и не становится записью model.Diagnostic.
type CheckFailure struct {
	// Code — код проверки, которая не смогла завершиться.
	Code diagnostics.Code
	// Err — первая причина сбоя Check или нарушения им API срабатываний.
	Err error
}

// Error возвращает сообщение «сбой диагностики V001: <причина>» с фактическим
// Code. Сообщение сохраняет идентификатор при обычном оборачивании через %w.
func (e *CheckFailure) Error() string {
	return fmt.Sprintf("сбой диагностики %s: %v", e.Code, e.Err)
}

// Unwrap возвращает исходную причину для errors.Is и errors.As.
func (e *CheckFailure) Unwrap() error {
	return e.Err
}

// Run принимает модель и неизменённую выборку checks из успешного Prepare
// для этого же документа; с момента Prepare app добавляет только Processing.
// Проверяет текущий Processing, создаёт закрытое представление Lines
// и выполняет Check в порядке checks. Повторно версию и реестр не просматривает,
// полный контроль входа не дублирует; итоговый CheckModel остаётся обязательным.
// Каждый Check получает новый накопитель. При первом техническом сбое
// возвращает *CheckFailure с V-кодом, отбрасывает срабатывания сбойной проверки
// и не запускает оставшиеся Check. app прекращает также обработку следующих файлов.
// Успешные срабатывания применяются к отдельной копии изменяемых частей модели;
// result обновляется только после успеха всех Check и итогового контроля.
// При err!=nil result не меняется; app не сохраняет этот файл.
// Любое число error/warning/recommendation не создаёт возвращаемую ошибку.
// nil result либо пустая выборка даёт err без изменения result; штатный путь
// Prepare до Run отсекает неподдерживаемую версию и пустую выборку.
func Run(result *model.Result, checks []diagnostics.Definition, processingID string) error {
	if result == nil {
		return fmt.Errorf("модель отсутствует")
	}
	if len(checks) == 0 {
		return fmt.Errorf("выборка диагностик пуста")
	}
	if err := checkCurrentProcessing(result, processingID); err != nil {
		return err
	}
	lines := newLinesView(result.Lines)
	candidate := copyForUpdate(result)
	for _, definition := range checks {
		occurrences, err := runCheck(lines, definition)
		if err != nil {
			return err
		}
		if err := appendOccurrences(candidate, processingID, occurrences); err != nil {
			return fmt.Errorf("применение срабатываний %s: %w", definition.Code, err)
		}
	}
	orderNewDiagnostics(candidate, processingID)
	setErrorFlags(candidate)
	if err := CheckModel(candidate); err != nil {
		return fmt.Errorf("итоговая целостность модели: %w", err)
	}
	*result = *candidate
	return nil
}

// checkCurrentProcessing проверяет, что processingID — ID последней записи,
// что он уникален и ещё не используется диагностиками; время задаёт app.
func checkCurrentProcessing(result *model.Result, processingID string) error {
	if result == nil || processingID == "" || len(result.Processing) == 0 {
		return fmt.Errorf("текущая запись processing отсутствует")
	}
	last := len(result.Processing) - 1
	if result.Processing[last].ID != processingID {
		return fmt.Errorf("processing %q не является последней записью", processingID)
	}
	for _, entry := range result.Processing[:last] {
		if entry.ID == processingID {
			return fmt.Errorf("processing ID %q повторяется", processingID)
		}
	}
	for _, entry := range result.Diagnostics {
		if entry.Source == processingID {
			return fmt.Errorf("processing %q уже используется диагностиками", processingID)
		}
	}
	return nil
}

// runCheck выполняет одну Check с новым collector и закрывает накопитель.
// При первой ошибке возвращает nil и *CheckFailure с Code текущего Definition;
// более ранняя ошибка накопителя имеет приоритет перед ошибкой возврата Check.
// Непредвиденный panic только внутри вызова Check преобразует в такой же отказ:
// это обработка дефекта с остановкой, а не штатный способ возврата ошибок.
// Ни частичные срабатывания, ни сам сбой в модель не переносятся. При успехе заполняет
// Code, Severity, Scope и Fatal=false по Definition, но ещё не ID/Source.
// Message начинается с Definition.Message; непустое сообщение срабатывания
// добавляется без изменений после «\nконкретизация: ».
func runCheck(lines *linesView, definition diagnostics.Definition) ([]model.Diagnostic, error) {
	collector := newCollector(lines.lines, definition.Scope)
	var checkErr error
	func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				if cause, ok := recovered.(error); ok {
					checkErr = fmt.Errorf("panic в Check: %w", cause)
				} else {
					checkErr = fmt.Errorf("panic в Check: %v", recovered)
				}
			}
		}()
		checkErr = definition.Check(lines, collector)
	}()
	occurrences, err := collector.finish()
	if err != nil {
		return nil, &CheckFailure{Code: definition.Code, Err: err}
	}
	if checkErr != nil {
		return nil, &CheckFailure{Code: definition.Code, Err: checkErr}
	}
	for i := range occurrences {
		entry := &occurrences[i]
		entry.DiagnosticCode = definition.Code
		entry.SeverityLevel = definition.Severity
		entry.DiagnosticScope = definition.Scope
		entry.Fatal = false
		message := definition.Message
		if entry.Message != "" {
			message += "\nконкретизация: " + entry.Message
		}
		entry.Message = message
	}
	return occurrences, nil
}
