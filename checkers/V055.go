package checkers

import (
	"strings"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV055 регистрирует ошибку размещения первого основного заголовка.
func registrationV055(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V055",
		Severity: diagnostics.SeverityError,
		Message:  "Основной заголовок находится в недопустимом месте.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV055,
	})
}

// checkV055 проверяет только первый кандидат с одним начальным #, в том числе
// ошибочный. Raw не нормализуется; диапазоны относятся к исходным Unicode-
// колонкам с 1 и исключающей правой границей. Повторные объявления не проверяются.
func checkV055(lines diagnostics.Lines, findings diagnostics.Findings) error {
	var heading, column, version int
	var nested bool
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		inBlock, literal := v055Context(lines, line)
		if literal != "" {
			continue
		}
		if version == 0 && v055Tag(line) == "dsl-version" {
			version = number
		}
		if heading == 0 {
			if start := v055HeadingColumn(line.Raw()); start != 0 {
				heading, column, nested = number, start, inBlock
			}
		}
	}
	if heading == 0 {
		return nil
	}

	var obstacle diagnostics.Line
	for number := 1; number < heading; number++ {
		line, _ := lines.Line(number)
		_, literal := v055Context(lines, line)
		if literal == "editor" || v055AllowedBefore(line) {
			continue
		}
		obstacle = line
		break
	}

	var reasons []string
	if nested {
		reasons = append(reasons, "Заголовок находится внутри фигурного блока")
	}
	if obstacle != nil {
		reasons = append(reasons, "Перед заголовком находится недопустимая конструкция")
	}
	if version > heading {
		reasons = append(reasons, "Заголовок предшествует первому объявлению @dsl-version")
	}
	if len(reasons) == 0 {
		return nil
	}
	occurrence, err := findings.Add(heading, column, heading, column+1, strings.Join(reasons, "; "))
	if err != nil {
		return err
	}
	if obstacle != nil {
		return findings.AddRelated(occurrence, obstacle.Number(), 1,
			obstacle.Number(), utf8.RuneCountInString(obstacle.Raw())+1)
	}
	return nil
}

// v055HeadingColumn возвращает исходную колонку одиночного начального # либо 0.
// Только U+0020 и U+0009 перед маркером пропускаются по DSL 1.14–1.15;
// экранирование и семантическое значение строки не преобразуются.
func v055HeadingColumn(raw string) int {
	start := 0
	for start < len(raw) && (raw[start] == ' ' || raw[start] == '\t') {
		start++
	}
	if start == len(raw) || raw[start] != '#' || start+1 < len(raw) && raw[start+1] == '#' {
		return 0
	}
	// Перед маркером только однобайтовые символы, каждый занимает одну колонку.
	return start + 1
}

// v055Context возвращает наличие фигурного предка и имя литерального блока.
// Логический родитель без block-start сам по себе не означает фигурный блок.
func v055Context(lines diagnostics.Lines, line diagnostics.Line) (inBlock bool, literal string) {
	for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
		line, _ = lines.Line(parent)
		if line.Type() != "block-start" {
			continue
		}
		inBlock = true
		if tag := v055Tag(line); tag == "editor" || tag == "multifill" {
			return true, tag
		}
	}
	return inBlock, ""
}

// v055Tag возвращает готовое каноническое имя тега либо пустую строку.
// Ошибочные объявления могут сохранять распознанный элемент tag.
func v055Tag(line diagnostics.Line) string {
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" {
			name, _ := element.Value()
			return name
		}
	}
	return ""
}

// v055AllowedBefore возвращает true для разрешённой предшествующей строки.
// Содержимое и закрытие editor исключаются отдельно по родительской цепочке.
func v055AllowedBefore(line diagnostics.Line) bool {
	if line.Type() == "blank" {
		return true
	}
	switch v055Tag(line) {
	case "dsl-version", "document-id", "section", "order", "resource-dir", "editor":
		return true
	default:
		return false
	}
}
