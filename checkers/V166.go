package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV166 регистрирует запрет answer и hint в нижнем списке matching.
func registrationV166(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V166",
		Severity: diagnostics.SeverityError,
		Message:  "Тег @answer или @hint размещён после разделителя ---.",
		Scope:    diagnostics.ScopeElement,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV166,
	})
}

// checkV166 использует только собственные строки каждого блока matching.
// Без единственного разделителя нижняя часть не определяется. Диапазон каждого
// сообщения совпадает с исходным tag, в Unicode-колонках с исключённым концом.
func checkV166(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		block, _ := lines.Line(number)
		if block.Type() != "block-start" || v166Tag(block, "matching") == nil {
			continue
		}
		children := v166Children(lines, block)
		count, separator := 0, 0
		for _, line := range children {
			if strings.Trim(line.Raw(), " \t") == "---" {
				count++
				separator = line.Number()
			}
		}
		if count != 1 {
			continue
		}
		for _, line := range children {
			if line.Number() <= separator {
				continue
			}
			tag := v166Tag(line, "answer", "hint")
			if tag == nil {
				continue
			}
			if _, err := findings.Add(line.Number(), tag.Start(), line.Number(), tag.End(), ""); err != nil {
				return err
			}
		}
	}
	return nil
}

// v166Children возвращает непосредственные строки до закрытия блока либо
// выхода из его глубины. Родители берутся из модели, Raw не разбирается заново.
func v166Children(lines diagnostics.Lines, block diagnostics.Line) []diagnostics.Line {
	var children []diagnostics.Line
	for number := block.Number() + 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if line.NestingLevel() <= block.NestingLevel() {
			break
		}
		parent, _ := line.ParentLine()
		if parent != block.Number() {
			continue
		}
		if line.Type() == "block-end" {
			break
		}
		children = append(children, line)
	}
	return children
}

// v166Tag находит распознанный элемент tag с одним из указанных имён.
// Обычный текст и экранированные псевдотеги не становятся объявлениями.
func v166Tag(line diagnostics.Line, names ...string) diagnostics.Element {
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() != "tag" {
			continue
		}
		value, _ := element.Value()
		for _, name := range names {
			if strings.EqualFold(value, name) {
				return element
			}
		}
	}
	return nil
}
