package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV120 регистрирует рекомендацию однострочной формы для DSL 1.2.
func registrationV120(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V120",
		Severity: diagnostics.SeverityRecommendation,
		Message:  "Блок без названия содержит одну строку и допускает однострочную запись.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV120,
	})
}

// checkV120 использует готовые родительские связи и исходные позиции тега.
// Вложенный блок проверяется самостоятельно, даже если исключён его родитель.
func checkV120(lines diagnostics.Lines, findings diagnostics.Findings) error {
	children := make(map[int][]int)
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if parent, found := line.ParentLine(); found {
			children[parent] = append(children[parent], number)
		}
	}
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		name, tag := v120Opening(line)
		if tag == nil || !v120SingleRow(lines, children[number], name) {
			continue
		}
		detail := "Сохраните текстовое значение в однострочной форме @" + name + " CONTENT, заново учитывая экранирование нового контекста."
		if name == "wordlist" {
			detail = "Сохраните текстовое значение как один элемент однострочного @wordlist: заново учтите экранирование нового контекста, экранируйте внутренние точки с запятой и оформите завершающую точку."
		}
		if _, err := findings.Add(number, tag.Start(), number, tag.End(), detail); err != nil {
			return err
		}
	}
	return nil
}

// v120Opening возвращает имя и tag блока без параметров; nil исключает блок.
// Между тегом и скобкой разрешён минимум один U+0020, но не табуляция.
func v120Opening(line diagnostics.Line) (string, diagnostics.Element) {
	if line.Type() != "block-start" {
		return "", nil
	}
	tag, opening := v120Element(line, "tag"), v120Element(line, "block-open")
	if tag == nil || opening == nil {
		return "", nil
	}
	name, _ := tag.Value()
	name = strings.ToLower(name)
	switch name {
	case "example", "wordlist", "instr", "note", "alt", "hint", "answer":
	default:
		return "", nil
	}
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "title" {
			return "", nil
		}
	}
	if opening.Start() <= tag.End() {
		return "", nil
	}
	gap := []rune(line.Raw())[tag.End()-1 : opening.Start()-1]
	for _, char := range gap {
		if char != ' ' {
			return "", nil
		}
	}
	return name, tag
}

// v120SingleRow возвращает true для закрытого блока с одной строкой текста.
// Неизвестное текстовое значение не доказывает возможность переноса. Raw
// используется для подсчёта строк и разделителей, Value повторно не декодируется.
func v120SingleRow(lines diagnostics.Lines, children []int, name string) bool {
	if len(children) == 0 {
		return false
	}
	closing, _ := lines.Line(children[len(children)-1])
	if closing.Type() != "block-end" || v120Element(closing, "block-close") == nil {
		return false
	}
	count := 0
	for _, number := range children[:len(children)-1] {
		line, _ := lines.Line(number)
		raw := strings.Trim(line.Raw(), " \t")
		if raw == "" {
			continue
		}
		if line.Type() != "content" || name == "example" && raw == "---" {
			return false // Включает вложенные объявления, разделители и лишние закрытия.
		}
		count++
		if count > 1 {
			return false
		}
		nonempty := false
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			value, present := element.Value()
			if element.Type() != "content" || !present {
				return false
			}
			nonempty = nonempty || strings.Trim(value, " \t") != ""
		}
		if !nonempty {
			return false
		}
	}
	return count == 1
}

// v120Element возвращает единственный элемент указанного типа.
// nil означает отсутствие либо неоднозначность структурного элемента.
func v120Element(line diagnostics.Line, kind string) diagnostics.Element {
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
