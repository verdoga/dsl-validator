package checkers

import (
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV131 регистрирует отсутствие определения фрагмента только для DSL 1.2.
func registrationV131(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V131",
		Severity: diagnostics.SeverityError,
		Message:  "Для @include не найден @fragment с указанным идентификатором.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV131,
	})
}

// checkV131 сначала собирает определения всего файла, затем проверяет обращения.
// Ветви, шаги и порядок объявлений не ограничивают поиск; дубликат ID остаётся
// существующим определением. Содержимое фрагментов не разворачивается.
func checkV131(lines diagnostics.Lines, findings diagnostics.Findings) error {
	defined := make(map[string]bool)
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if line.Type() != "block-start" || v131Literal(lines, line) {
			continue
		}
		if tag := v131Tag(line, "fragment"); tag != nil {
			id, _, _ := v131ID(line, tag)
			if id != "" {
				defined[strings.ToLower(id)] = true
			}
		}
	}
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if v131Literal(lines, line) {
			continue
		}
		tag := v131Tag(line, "include")
		if tag == nil {
			continue
		}
		id, start, end := v131ID(line, tag)
		if id == "" || defined[strings.ToLower(id)] {
			continue
		}
		if _, err := findings.Add(number, start, number, end, ""); err != nil {
			return err
		}
	}
	return nil
}

// v131Tag возвращает распознанный тег с данным каноническим именем либо nil.
// Полностью неразобранная строка не классифицируется повторно по Raw.
func v131Tag(line diagnostics.Line, name string) diagnostics.Element {
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" {
			value, _ := element.Value()
			if value == name {
				return element
			}
		}
	}
	return nil
}

// v131ID выделяет первый служебный аргумент после распознанного тега.
// Возвращает допустимый ID и исходные Unicode-колонки с 1, конец исключён;
// пустой ID означает отсутствие либо недопустимость аргумента.
// Крайние U+0020/U+0009 строки учитываются по п. 1.14 DSL, а разделителем
// аргументов служит только U+0020. Сам ID не обрезается и не декодируется.
func v131ID(line diagnostics.Line, tag diagnostics.Element) (id string, start, end int) {
	raw := []rune(line.Raw())
	limit := len(raw)
	for limit > tag.End()-1 && (raw[limit-1] == ' ' || raw[limit-1] == '\t') {
		limit--
	}
	first := tag.End() - 1
	if first >= limit || raw[first] != ' ' {
		return "", 0, 0
	}
	for first < limit && raw[first] == ' ' {
		first++
	}
	last := first
	for last < limit && raw[last] != ' ' {
		last++
	}
	id, start, end = string(raw[first:last]), first+1, last+1
	if !v131ValidID(id) {
		return "", 0, 0
	}
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "identifier" && element.Start() == start && element.End() == end {
			value, present := element.Value()
			if !present || !v131ValidID(value) {
				return "", 0, 0
			}
			return value, start, end
		}
	}
	return id, start, end
}

// v131ValidID допускает только непустые ID из Letter, Nd и ASCII-знаков . - _.
func v131ValidID(id string) bool {
	if id == "" {
		return false
	}
	for _, char := range id {
		if !unicode.IsLetter(char) && !unicode.IsDigit(char) && char != '.' && char != '-' && char != '_' {
			return false
		}
	}
	return true
}

// v131Literal исключает потомков литеральных editor и multifill по готовым связям.
func v131Literal(lines diagnostics.Lines, line diagnostics.Line) bool {
	for {
		parent, found := line.ParentLine()
		if !found {
			return false
		}
		line, _ = lines.Line(parent)
		if line.Type() == "block-start" && (v131Tag(line, "editor") != nil || v131Tag(line, "multifill") != nil) {
			return true
		}
	}
}
