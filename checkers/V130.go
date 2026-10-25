package checkers

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// v130Position хранит исходный диапазон ID в Unicode-колонках с 1.
// Правая граница исключается.
type v130Position struct {
	line, start, end int
}

// v130Group сохраняет ключ сравнения и все определения в порядке исходника.
type v130Group struct {
	id        string
	positions []v130Position
}

// registrationV130 регистрирует проверку ID фрагментов только для DSL 1.2.
func registrationV130(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V130",
		Severity: diagnostics.SeverityError,
		Message:  "Идентификатор фрагмента повторяется в документе.",
		Scope:    diagnostics.ScopeDocument,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV130,
	})
}

// checkV130 группирует определения во всём файле, включая разные ветви.
// Группы выдаются по первому определению, без основной позиции и без развёртки include.
func checkV130(lines diagnostics.Lines, findings diagnostics.Findings) error {
	indices := make(map[string]int)
	var groups []v130Group
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		identifier := v130Identifier(line)
		if identifier == nil || v130Literal(lines, line) {
			continue
		}
		value, present := identifier.Value()
		if !present || !v130ValidID(value) {
			continue
		}
		key := strings.ToLower(value)
		index, found := indices[key]
		if !found {
			index = len(groups)
			indices[key] = index
			groups = append(groups, v130Group{id: key})
		}
		groups[index].positions = append(groups[index].positions,
			v130Position{line.Number(), identifier.Start(), identifier.End()})
	}
	for _, group := range groups {
		if len(group.positions) < 2 {
			continue
		}
		detail := fmt.Sprintf("ID: «%s». Число определений: %d.", group.id, len(group.positions))
		occurrence, err := findings.Add(0, 0, 0, 0, detail)
		if err != nil {
			return err
		}
		for _, position := range group.positions {
			if err := findings.AddRelated(occurrence, position.line, position.start, position.line, position.end); err != nil {
				return err
			}
		}
	}
	return nil
}

// v130Identifier возвращает однозначно выделенный ID начального fragment.
// Ошибка в другой части строки не исключает определение; из Raw ID не восстанавливается.
func v130Identifier(line diagnostics.Line) diagnostics.Element {
	tag := v130Element(line, "tag")
	identifier := v130Element(line, "identifier")
	if tag == nil || identifier == nil || identifier.Start() < tag.End() {
		return nil
	}
	name, _ := tag.Value()
	if name != "fragment" {
		return nil
	}
	// Отступ проверяется без изменения исходника. Экранированная запись
	// и упоминание посреди текста не являются объявлением.
	raw := line.Raw()
	start := 0
	for start < len(raw) && (raw[start] == ' ' || raw[start] == '\t') {
		start++
	}
	if start == len(raw) || raw[start] != '@' || tag.Start() != start+1 {
		return nil
	}
	return identifier
}

// v130Element возвращает единственный элемент указанного типа.
// nil означает отсутствие либо неоднозначность, без выбора первого кандидата.
func v130Element(line diagnostics.Line, kind string) diagnostics.Element {
	var found diagnostics.Element
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == kind {
			if found != nil {
				return nil
			}
			found = element
		}
	}
	return found
}

// v130Literal исключает содержимое литеральных блоков по готовым родителям.
func v130Literal(lines diagnostics.Lines, line diagnostics.Line) bool {
	for {
		parent, found := line.ParentLine()
		if !found {
			return false
		}
		line, _ = lines.Line(parent)
		if line.Type() == "block-start" {
			if tag := v130Element(line, "tag"); tag != nil {
				name, _ := tag.Value()
				if name == "editor" || name == "multifill" {
					return true
				}
			}
		}
	}
}

// v130ValidID проверяет полное Value без обрезки, декодирования или нормализации.
func v130ValidID(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if !unicode.IsLetter(char) && !unicode.IsDigit(char) && char != '.' && char != '-' && char != '_' {
			return false
		}
	}
	return true
}
