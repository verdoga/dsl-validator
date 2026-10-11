package validator

import (
	"fmt"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
)

// makeLocation переводит четыре числа в существующий model.Location.
// Четыре нуля дают nil; частично нулевые и отрицательные координаты — ошибку.
// Допустимость nil для конкретной области проверяет checkOccurrence.
func makeLocation(startLine, startColumn, endLine, endColumn int) (*model.Location, error) {
	if startLine == 0 && startColumn == 0 && endLine == 0 && endColumn == 0 {
		return nil, nil
	}
	if startLine < 1 || startColumn < 1 || endLine < 1 || endColumn < 1 {
		return nil, fmt.Errorf("координаты диапазона должны быть положительными либо все равны нулю")
	}
	return &model.Location{
		Start: model.Position{Line: startLine, Column: startColumn},
		End:   model.Position{Line: endLine, Column: endColumn},
	}, nil
}

// checkLocation проверяет существование строк, диапазон Unicode-колонок,
// порядок позиций и исключающую правую границу; пустая позиция допустима.
func checkLocation(lines []model.Line, location model.Location) error {
	for _, position := range [...]model.Position{location.Start, location.End} {
		if position.Line < 1 || position.Line > len(lines) {
			return fmt.Errorf("строка %d диапазона не существует", position.Line)
		}
		if position.Column < 1 || position.Column-1 > utf8.RuneCountInString(lines[position.Line-1].Raw) {
			return fmt.Errorf("колонка %d выходит за границы строки %d", position.Column, position.Line)
		}
	}
	if location.End.Line < location.Start.Line ||
		(location.End.Line == location.Start.Line && location.End.Column < location.Start.Column) {
		return fmt.Errorf("конец диапазона предшествует началу")
	}
	return nil
}

// checkOccurrence проверяет обязательность Location, соответствие Scope,
// связанные диапазоны и точное совпадение с элементом для ScopeElement.
// Для новых ScopeLine и ScopeBlock отсутствие Location запрещено.
func checkOccurrence(lines []model.Line, scope diagnostics.Scope, occurrence model.Diagnostic) error {
	switch scope {
	case diagnostics.ScopeElement, diagnostics.ScopeLine, diagnostics.ScopeBlock, diagnostics.ScopeDocument:
	default:
		return fmt.Errorf("неизвестная область срабатывания %q", scope)
	}
	location := occurrence.Location
	if location == nil {
		if scope != diagnostics.ScopeDocument {
			return fmt.Errorf("область %q требует основной диапазон", scope)
		}
	} else {
		if err := checkLocation(lines, *location); err != nil {
			return fmt.Errorf("основной диапазон: %w", err)
		}
		if scope == diagnostics.ScopeLine || scope == diagnostics.ScopeElement {
			if location.Start.Line != location.End.Line {
				return fmt.Errorf("диапазон области %q должен находиться в одной строке", scope)
			}
		}
		if scope == diagnostics.ScopeElement {
			found := false
			for _, element := range lines[location.Start.Line-1].Elements {
				if element.Start == location.Start.Column && element.End == location.End.Column {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("основной диапазон не совпадает с диапазоном существующего элемента")
			}
		}
	}
	seen := make(map[model.Location]struct{}, len(occurrence.RelatedLocations))
	for i, related := range occurrence.RelatedLocations {
		if err := checkLocation(lines, related); err != nil {
			return fmt.Errorf("связанный диапазон %d: %w", i+1, err)
		}
		if location != nil && related == *location {
			return fmt.Errorf("связанный диапазон %d совпадает с основным", i+1)
		}
		if _, exists := seen[related]; exists {
			return fmt.Errorf("связанный диапазон %d повторяется", i+1)
		}
		seen[related] = struct{}{}
	}
	return nil
}
