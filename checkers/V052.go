package checkers

import (
	"strings"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV052 регистрирует проверку расположения второго заголовка в DSL 1.2.
func registrationV052(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V052",
		Severity: diagnostics.SeverityError,
		Message:  "Заголовок второго уровня находится в недопустимом месте.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV052,
	})
}

// checkV052 проверяет положение кандидатов ##, включая ошибочные строки.
// Отсутствие основного заголовка само по себе не является причиной V052.
// Диапазоны охватывают исходные строки без EOL; правая граница исключается.
func checkV052(lines diagnostics.Lines, findings diagnostics.Findings) error {
	main := 0
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if v052HeadingLevel(line) != 1 {
			continue
		}
		nested, _ := v052Context(lines, line)
		if !nested {
			main = number
			break
		}
	}

	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if v052HeadingLevel(line) != 2 {
			continue
		}
		nested, literal := v052Context(lines, line)
		if literal {
			continue
		}
		var reasons []string
		if nested {
			reasons = append(reasons, "Заголовок вложен в фигурный блок.")
		}
		var obstacle diagnostics.Line
		if main > number {
			reasons = append(reasons, "Заголовок расположен до основного заголовка.")
		} else if main != 0 {
			for between := main + 1; between < number; between++ {
				candidate, _ := lines.Line(between)
				if !v052AllowedBetween(lines, candidate) {
					obstacle = candidate
					reasons = append(reasons, "Перед заголовком находится недопустимая конструкция.")
					break
				}
			}
		}
		if len(reasons) == 0 {
			continue
		}
		occurrence, err := findings.Add(number, 1, number,
			utf8.RuneCountInString(line.Raw())+1, strings.Join(reasons, " "))
		if err != nil {
			return err
		}
		if obstacle != nil {
			if err := findings.AddRelated(occurrence, obstacle.Number(), 1,
				obstacle.Number(), utf8.RuneCountInString(obstacle.Raw())+1); err != nil {
				return err
			}
		}
	}
	return nil
}

// v052HeadingLevel считает начальные # после U+0020/U+0009 по DSL 1.14–1.15.
// Raw не декодируется: экранированное начало не становится кандидатом.
// Разделитель и название намеренно не проверяются даже у invalid/unparsed.
func v052HeadingLevel(line diagnostics.Line) int {
	raw := line.Raw()
	start := 0
	for start < len(raw) && (raw[start] == ' ' || raw[start] == '\t') {
		start++
	}
	end := start
	for end < len(raw) && raw[end] == '#' {
		end++
	}
	return end - start
}

// v052Context возвращает наличие фигурного предка и литерального контекста.
// Логический родитель-заголовок или task сам по себе не является фигурным блоком.
func v052Context(lines diagnostics.Lines, line diagnostics.Line) (nested, literal bool) {
	for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
		line, _ = lines.Line(parent)
		if line.Type() != "block-start" {
			continue
		}
		nested = true
		tag := v052Tag(line)
		if tag == "editor" || tag == "multifill" {
			literal = true
		}
	}
	return nested, literal
}

// v052Tag читает каноническое имя из модели, не распознавая объявления по Raw.
func v052Tag(line diagnostics.Line) string {
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" {
			value, _ := element.Value()
			return value
		}
	}
	return ""
}

// v052AllowedBetween разрешает строку в промежутке между основным и вторым
// заголовками. Потомки editor/fragment, включая block-end, пропускаются целиком.
// Это не исключает кандидаты внутри fragment из самостоятельной проверки.
func v052AllowedBetween(lines diagnostics.Lines, line diagnostics.Line) bool {
	if line.Type() == "blank" {
		return true
	}
	tag := v052Tag(line)
	if line.Type() == "block-start" && (tag == "editor" || tag == "fragment") ||
		line.Type() == "tag" && tag == "editor" {
		return true
	}
	for ancestor := line; ; {
		parent, found := ancestor.ParentLine()
		if !found {
			break
		}
		ancestor, _ = lines.Line(parent)
		if ancestor.Type() == "block-start" {
			tag := v052Tag(ancestor)
			if tag == "editor" || tag == "fragment" {
				return true
			}
		}
	}
	if v052HeadingLevel(line) == 2 {
		_, literal := v052Context(lines, line)
		return !literal
	}
	return false
}
