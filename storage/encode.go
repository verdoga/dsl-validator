package storage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/verdoga/dsl-validator/model"
)

// encodeUpdate готовит полный JSON на основе snapshot.original.
// Меняет только новые суффиксы processing/diagnostics, document.hasErrors,
// lines[].hasErrors и дополненные elements[].errorIds.
// Предусловие: остальные известные поля и прежние записи не менялись.
// Неизвестные поля на любом уровне и значения прежних записей сохраняются.
// Результат имеет отступ два пробела и конечный LF; побайтовое форматирование
// всего исходного JSON не обещается. Экспортируемые строки не HTML-экранируются.
func encodeUpdate(snapshot *Snapshot, result *model.Result) ([]byte, error) {
	if snapshot == nil || result == nil {
		return nil, fmt.Errorf("для сохранения требуются снимок и модель")
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(snapshot.original, &root); err != nil {
		return nil, fmt.Errorf("не удалось разобрать исходный JSON: %w", err)
	}
	if root == nil {
		return nil, fmt.Errorf("исходный JSON должен быть объектом")
	}
	processing, err := appendProcessing(root["processing"], result.Processing)
	if err != nil {
		return nil, fmt.Errorf("processing: %w", err)
	}
	diagnostics, err := appendDiagnostics(root["diagnostics"], result.Diagnostics)
	if err != nil {
		return nil, fmt.Errorf("diagnostics: %w", err)
	}
	document, err := updateDocumentFlag(root["document"], result.Document.HasErrors)
	if err != nil {
		return nil, fmt.Errorf("document: %w", err)
	}
	lines, err := updateLineFlags(root["lines"], result.Lines)
	if err != nil {
		return nil, fmt.Errorf("lines: %w", err)
	}
	root["processing"], root["diagnostics"] = processing, diagnostics
	root["document"], root["lines"] = document, lines
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(root); err != nil {
		return nil, fmt.Errorf("не удалось сформировать JSON: %w", err)
	}
	return output.Bytes(), nil
}

// appendProcessing добавляет к сырому массиву только новые Processing;
// прежние объекты, включая неизвестные поля, переносит без пересоздания.
func appendProcessing(original json.RawMessage, entries []model.Processing) (json.RawMessage, error) {
	var history []json.RawMessage
	if err := json.Unmarshal(original, &history); err != nil {
		return nil, fmt.Errorf("не удалось прочитать историю: %w", err)
	}
	if history == nil {
		return nil, fmt.Errorf("история должна быть массивом, а не null")
	}
	if len(entries) < len(history) {
		return nil, fmt.Errorf("нельзя сократить историю с %d до %d записей", len(history), len(entries))
	}
	for _, entry := range entries[len(history):] {
		var encoded bytes.Buffer
		encoder := json.NewEncoder(&encoded)
		encoder.SetEscapeHTML(false)
		if err := encoder.Encode(entry); err != nil {
			return nil, fmt.Errorf("не удалось записать запуск: %w", err)
		}
		history = append(history, json.RawMessage(encoded.Bytes()))
	}
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(history); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

// appendDiagnostics добавляет только новые Diagnostics, сохраняя прежние
// объекты и неизвестные поля; сокращение прежнего массива отвергает.
func appendDiagnostics(original json.RawMessage, entries []model.Diagnostic) (json.RawMessage, error) {
	var history []json.RawMessage
	if err := json.Unmarshal(original, &history); err != nil {
		return nil, fmt.Errorf("не удалось прочитать диагностики: %w", err)
	}
	if history == nil {
		return nil, fmt.Errorf("диагностики должны быть массивом, а не null")
	}
	if len(entries) < len(history) {
		return nil, fmt.Errorf("нельзя сократить диагностики с %d до %d записей", len(history), len(entries))
	}
	for _, entry := range entries[len(history):] {
		var encoded bytes.Buffer
		encoder := json.NewEncoder(&encoded)
		encoder.SetEscapeHTML(false)
		if err := encoder.Encode(entry); err != nil {
			return nil, fmt.Errorf("не удалось записать диагностику: %w", err)
		}
		history = append(history, json.RawMessage(encoded.Bytes()))
	}
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(history); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

// updateDocumentFlag заменяет только document.hasErrors в исходном объекте.
func updateDocumentFlag(original json.RawMessage, hasErrors bool) (json.RawMessage, error) {
	var document map[string]json.RawMessage
	if err := json.Unmarshal(original, &document); err != nil {
		return nil, fmt.Errorf("не удалось прочитать документ: %w", err)
	}
	if document == nil {
		return nil, fmt.Errorf("документ должен быть объектом, а не null")
	}
	document["hasErrors"] = json.RawMessage(strconv.FormatBool(hasErrors))
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(document); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

// updateLineFlags переносит флаги строк и дополненные ErrorIDs, сохраняя
// все остальные поля строк и элементов. Изменение количества строк/элементов
// отвергает; порядок и содержимое строк являются предусловием Save.
func updateLineFlags(original json.RawMessage, lines []model.Line) (json.RawMessage, error) {
	var originals []map[string]json.RawMessage
	if err := json.Unmarshal(original, &originals); err != nil {
		return nil, fmt.Errorf("не удалось прочитать строки: %w", err)
	}
	if originals == nil {
		return nil, fmt.Errorf("строки должны быть массивом, а не null")
	}
	if len(originals) != len(lines) {
		return nil, fmt.Errorf("число строк изменилось с %d на %d", len(originals), len(lines))
	}
	for i, originalLine := range originals {
		if originalLine == nil {
			return nil, fmt.Errorf("строка %d должна быть объектом", i+1)
		}
		var elements []map[string]json.RawMessage
		if err := json.Unmarshal(originalLine["elements"], &elements); err != nil {
			return nil, fmt.Errorf("элементы строки %d: %w", i+1, err)
		}
		if elements == nil || len(elements) != len(lines[i].Elements) {
			return nil, fmt.Errorf("строка %d: ожидался исходный массив из %d элементов", i+1, len(elements))
		}
		for j, element := range elements {
			if element == nil {
				return nil, fmt.Errorf("строка %d: элемент %d должен быть объектом", i+1, j)
			}
			var ids bytes.Buffer
			encoder := json.NewEncoder(&ids)
			encoder.SetEscapeHTML(false)
			if err := encoder.Encode(lines[i].Elements[j].ErrorIDs); err != nil {
				return nil, fmt.Errorf("строка %d, элемент %d: %w", i+1, j, err)
			}
			element["errorIds"] = json.RawMessage(ids.Bytes())
		}
		var encoded bytes.Buffer
		encoder := json.NewEncoder(&encoded)
		encoder.SetEscapeHTML(false)
		if err := encoder.Encode(elements); err != nil {
			return nil, fmt.Errorf("элементы строки %d: %w", i+1, err)
		}
		originalLine["elements"] = json.RawMessage(encoded.Bytes())
		originalLine["hasErrors"] = json.RawMessage(strconv.FormatBool(lines[i].HasErrors))
	}
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(originals); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}
