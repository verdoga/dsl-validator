package validator

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
)

// copyForUpdate копирует Result и все изменяемые здесь вложенные части:
// Diagnostics с диапазонами, Lines, Elements и ErrorIDs.
// Неизменяемые Document-поля и Processing не переписывает; общей изменяемой
// памяти с исходной моделью в перечисленных частях не оставляет.
func copyForUpdate(result *model.Result) *model.Result {
	candidate := *result
	if result.Diagnostics != nil {
		candidate.Diagnostics = make([]model.Diagnostic, len(result.Diagnostics))
	}
	for i, entry := range result.Diagnostics {
		candidate.Diagnostics[i] = entry
		if entry.Location != nil {
			location := *entry.Location
			candidate.Diagnostics[i].Location = &location
		}
		if entry.RelatedLocations != nil {
			candidate.Diagnostics[i].RelatedLocations = make([]model.Location, len(entry.RelatedLocations))
			copy(candidate.Diagnostics[i].RelatedLocations, entry.RelatedLocations)
		}
	}
	if result.Lines != nil {
		candidate.Lines = make([]model.Line, len(result.Lines))
	}
	for i, line := range result.Lines {
		candidate.Lines[i] = line
		if line.ParentLine != nil {
			parent := *line.ParentLine
			candidate.Lines[i].ParentLine = &parent
		}
		if line.Elements != nil {
			candidate.Lines[i].Elements = make([]model.Element, len(line.Elements))
		}
		for j, element := range line.Elements {
			candidate.Lines[i].Elements[j] = element
			if element.Value != nil {
				value := *element.Value
				candidate.Lines[i].Elements[j].Value = &value
			}
			if element.ErrorIDs != nil {
				candidate.Lines[i].Elements[j].ErrorIDs = make([]string, len(element.ErrorIDs))
				copy(candidate.Lines[i].Elements[j].ErrorIDs, element.ErrorIDs)
			}
		}
	}
	return &candidate
}

// appendOccurrences назначает ID и Source новым записям, добавляет их
// к кандидату и формирует обратные ссылки только для ScopeElement.
// Диагностики прежних запусков и их ID не изменяет и не удаляет.
func appendOccurrences(result *model.Result, processingID string, occurrences []model.Diagnostic) error {
	for _, entry := range occurrences {
		entry.ID = nextDiagnosticID(result.Diagnostics)
		entry.Source = processingID
		if entry.DiagnosticScope == diagnostics.ScopeElement {
			if err := linkElementError(result.Lines, entry); err != nil {
				return err
			}
		}
		result.Diagnostics = append(result.Diagnostics, entry)
	}
	return nil
}

// nextDiagnosticID выбирает свободный dN, N>=1, среди всех Diagnostics;
// номер запуска не используется как предположение об отсутствии совпадений.
func nextDiagnosticID(entries []model.Diagnostic) string {
	used := make(map[string]bool, len(entries))
	for _, entry := range entries {
		used[entry.ID] = true
	}
	for number := 1; ; number++ {
		id := "d" + strconv.Itoa(number)
		if !used[id] {
			return id
		}
	}
}

// linkElementError добавляет ID к существующему элементу с точным диапазоном.
// Для одинаковых пустых диапазонов выбирает первый подходящий элемент.
// Повторную ссылку не добавляет; отсутствие элемента — ошибка инварианта.
func linkElementError(lines []model.Line, diagnostic model.Diagnostic) error {
	location := diagnostic.Location
	if location == nil || location.Start.Line != location.End.Line || location.Start.Line < 1 || location.Start.Line > len(lines) {
		return fmt.Errorf("диагностика %q: диапазон не указывает на существующий элемент", diagnostic.ID)
	}
	for i := range lines[location.Start.Line-1].Elements {
		element := &lines[location.Start.Line-1].Elements[i]
		if element.Start != location.Start.Column || element.End != location.End.Column {
			continue
		}
		for _, id := range element.ErrorIDs {
			if id == diagnostic.ID {
				return nil
			}
		}
		element.ErrorIDs = append(element.ErrorIDs, diagnostic.ID)
		return nil
	}
	return fmt.Errorf("диагностика %q: элемент с точным диапазоном отсутствует", diagnostic.ID)
}

// orderNewDiagnostics устойчиво упорядочивает только новые записи по позиции:
// nil Location первой, затем начальная строка, колонка и порядок обнаружения.
// Прежний префикс Diagnostics сохраняет; ErrorIDs упорядочивает по итоговому
// порядку Diagnostics без изменения существующих ID и их относительного порядка.
func orderNewDiagnostics(result *model.Result, processingID string) {
	start := len(result.Diagnostics)
	for i, entry := range result.Diagnostics {
		if entry.Source == processingID {
			start = i
			break
		}
	}
	entries := result.Diagnostics[start:]
	sort.SliceStable(entries, func(i, j int) bool {
		left, right := entries[i].Location, entries[j].Location
		if left == nil {
			return right != nil
		}
		if right == nil {
			return false
		}
		if left.Start.Line != right.Start.Line {
			return left.Start.Line < right.Start.Line
		}
		return left.Start.Column < right.Start.Column
	})
	indices := make(map[string]int, len(result.Diagnostics))
	for i, entry := range result.Diagnostics {
		indices[entry.ID] = i
	}
	for i := range result.Lines {
		for j := range result.Lines[i].Elements {
			ids := result.Lines[i].Elements[j].ErrorIDs
			sort.SliceStable(ids, func(i, j int) bool { return indices[ids[i]] < indices[ids[j]] })
		}
	}
}

// setErrorFlags вычисляет Document.HasErrors и Line.HasErrors по всем запускам.
// Только severity=error устанавливает признак. Строка отмечается по началу
// основной Location либо её ErrorIDs; связанные места, родители и дети не влияют.
func setErrorFlags(result *model.Result) {
	document, lines := errorFlags(result)
	result.Document.HasErrors = document
	for i := range result.Lines {
		result.Lines[i].HasErrors = lines[result.Lines[i].Number]
	}
}
