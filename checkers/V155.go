package checkers

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV155 регистрирует предупреждение о смешанном режиме multifill.
func registrationV155(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code: "V155", Severity: diagnostics.SeverityWarning,
		Message:  "В @multifill смешаны пропуски с ответами и без ответов.",
		Scope:    diagnostics.ScopeBlock,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV155,
	})
}

// v155Gap хранит исходный диапазон обычного пропуска; конец исключается.
type v155Gap struct {
	line, start, end int
}

// checkV155 независимо проверяет всё собственное предзаполненное содержимое.
// Неоднозначность даже после обнаруженного смешения отменяет предупреждение.
func checkV155(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		opening, _ := lines.Line(number)
		tag := v155BlockTag(opening)
		if tag == nil {
			continue
		}
		name, _ := tag.Value()
		if name != "multifill" || v155LiteralParent(lines, opening) {
			continue
		}
		var ordinary []v155Gap
		special, ambiguous := false, false
		endLine, endColumn := number, utf8.RuneCountInString(opening.Raw())+1
		for next := number + 1; next <= lines.Len(); next++ {
			line, _ := lines.Line(next)
			parent, _ := line.ParentLine()
			if parent != number {
				break
			}
			endLine, endColumn = next, utf8.RuneCountInString(line.Raw())+1
			if line.Type() == "block-end" {
				for index := 0; index < line.ElementCount(); index++ {
					element, _ := line.Element(index)
					if element.Type() == "block-close" {
						endColumn = element.End()
						break
					}
				}
				break
			}
			gaps, hasSpecial, invalid := v155Scan(line.Raw(), next)
			ordinary = append(ordinary, gaps...)
			special = special || hasSpecial
			ambiguous = ambiguous || invalid
		}
		if ambiguous || !special || len(ordinary) == 0 {
			continue
		}
		occurrence, err := findings.Add(number, tag.Start(), endLine, endColumn,
			"Весь блок будет единым многострочным полем без автоматической проверки. Проверьте полноту служебных ответов; отсутствующие ответы нельзя придумывать")
		if err != nil {
			return err
		}
		for _, gap := range ordinary {
			if err := findings.AddRelated(occurrence, gap.line, gap.start, gap.line, gap.end); err != nil {
				return err
			}
		}
	}
	return nil
}

// v155BlockTag возвращает тег только разобранного начала блока.
func v155BlockTag(line diagnostics.Line) diagnostics.Element {
	if line.Type() != "block-start" {
		return nil
	}
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" {
			return element
		}
	}
	return nil
}

// v155LiteralParent исключает объявления, находящиеся в литеральной области.
func v155LiteralParent(lines diagnostics.Lines, line diagnostics.Line) bool {
	for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
		line, _ = lines.Line(parent)
		if tag := v155BlockTag(line); tag != nil {
			name, _ := tag.Value()
			if name == "editor" || name == "multifill" {
				return true
			}
		}
	}
	return false
}

// v155Scan возвращает обычные пропуски, наличие специального и неоднозначность.
// Разбор ведётся по исходным рунам: декодирование не создаёт новых конструкций.
func v155Scan(raw string, number int) (ordinary []v155Gap, special, ambiguous bool) {
	text := []rune(raw)
	for index := 0; index < len(text); {
		if text[index] == '\\' && index+1 < len(text) {
			if text[index+1] == '\\' {
				index += 2
				continue
			}
			if text[index+1] == '_' {
				index += 2
				for index < len(text) && text[index] == '_' {
					index++
				}
				continue
			}
		}
		if text[index] != '_' {
			index++
			continue
		}
		start := index
		for index < len(text) && text[index] == '_' {
			index++
		}
		if index-start != 5 || v155SeparatedAnswer(text, index) {
			return ordinary, special, true
		}
		if index == len(text) || text[index] != '{' {
			ordinary = append(ordinary, v155Gap{number, start + 1, index + 1})
			continue
		}
		// Все непосредственно соседние группы относятся к одному пропуску.
		for index < len(text) && text[index] == '{' {
			var valid bool
			index, valid = v155Answer(text, index+1)
			if !valid || v155SeparatedAnswer(text, index) {
				return ordinary, special, true
			}
		}
		special = true
	}
	return ordinary, special, false
}

// v155SeparatedAnswer обнаруживает пробельный промежуток перед активной скобкой.
func v155SeparatedAnswer(text []rune, index int) bool {
	start := index
	for index < len(text) && unicode.Is(unicode.White_Space, text[index]) {
		index++
	}
	return index > start && index < len(text) && text[index] == '{'
}

// v155Answer читает одну группу до первой неэкранированной закрывающей скобки.
// Возвращает индекс после неё и true только для корректного непустого ответа.
// Экранирование снимается однократно, только для скобок и обратного слеша.
func v155Answer(text []rune, index int) (int, bool) {
	var answer strings.Builder
	for index < len(text) {
		char := text[index]
		if char == '\\' && index+1 < len(text) && (text[index+1] == '{' || text[index+1] == '}' || text[index+1] == '\\') {
			answer.WriteRune(text[index+1])
			index += 2
			continue
		}
		if char == '{' {
			return index, false
		}
		if char == '}' {
			return index + 1, v155Normalize(answer.String()) != ""
		}
		answer.WriteRune(char)
		index++
	}
	return index, false
}

// v155Normalize выполняет основную нормализацию, сохраняя пунктуацию.
func v155Normalize(answer string) string {
	return strings.Join(strings.FieldsFunc(strings.ToLower(answer), func(char rune) bool {
		return unicode.Is(unicode.White_Space, char)
	}), " ")
}
