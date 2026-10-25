package checkers

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV179 регистрирует частичное заполнение ответов ordering.
func registrationV179(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code: "V179", Severity: diagnostics.SeverityError,
		Message:  "В @ordering ответы заданы только для части элементов.",
		Scope:    diagnostics.ScopeBlock,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV179,
	})
}

// checkV179 определяет связи самостоятельно, не оценивая значения ответов.
func checkV179(lines diagnostics.Lines, findings diagnostics.Findings) error {
	ends := make(map[int]int)
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if line.Type() == "block-end" {
			if parent, found := line.ParentLine(); found {
				if _, exists := ends[parent]; exists {
					ends[parent] = -1
				} else {
					ends[parent] = number
				}
			}
		}
	}
	for number := 1; number <= lines.Len(); number++ {
		block, _ := lines.Line(number)
		if block.Type() != "block-start" || v179Name(block) != "ordering" {
			continue
		}
		children, last, known := v179Body(lines, block, ends)
		if !known {
			continue
		}
		missing := v179Missing(children)
		if len(missing) == 0 {
			continue
		}
		occurrence, err := findings.Add(number, 1, last.Number(), utf8.RuneCountInString(last.Raw())+1, "")
		if err != nil {
			return err
		}
		for _, line := range missing {
			if err := findings.AddRelated(occurrence, line.Number(), 1, line.Number(), utf8.RuneCountInString(line.Raw())+1); err != nil {
				return err
			}
		}
	}
	return nil
}

// v179Body возвращает непосредственные значимые конструкции и последнюю строку
// блока. Без закрытия используется доступная часть по родителям модели.
// Неоднозначные закрытия и разорванная принадлежность дают known=false.
func v179Body(lines diagnostics.Lines, block diagnostics.Line, ends map[int]int) (children []diagnostics.Line, last diagnostics.Line, known bool) {
	last = block
	for number := block.Number() + 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if !v179Descendant(lines, line, block.Number()) {
			break
		}
		last = line
		parent, _ := line.ParentLine()
		if parent != block.Number() {
			continue
		}
		if line.Type() == "block-end" {
			if strings.Trim(line.Raw(), " \t") != "}" {
				return nil, last, false
			}
			break
		}
		if strings.Trim(line.Raw(), " \t") != "" {
			children = append(children, line)
		}
	}
	if end := ends[block.Number()]; end == -1 || (end != 0 && end != last.Number()) {
		return nil, last, false
	}
	for number := block.Number() + 1; number <= last.Number(); number++ {
		line, _ := lines.Line(number)
		if line.Type() == "block-start" && (ends[number] == -1 || ends[number] > last.Number()) {
			return nil, last, false
		}
		ancestor := line
		for parent, found := ancestor.ParentLine(); found && parent != block.Number(); parent, found = ancestor.ParentLine() {
			ancestor, _ = lines.Line(parent)
			if ancestor.Type() == "block-start" && ends[parent] != 0 && ends[parent] < number {
				return nil, last, false
			}
		}
	}
	return children, last, true
}

func v179Descendant(lines diagnostics.Lines, line diagnostics.Line, block int) bool {
	for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
		if parent == block {
			return true
		}
		if parent < block {
			return false
		}
		line, _ = lines.Line(parent)
	}
	return false
}

// v179Missing возвращает только доказанное частичное заполнение. Неправильно
// расположенный непосредственный answer подавляет V179 для всего блока.
func v179Missing(children []diagnostics.Line) []diagnostics.Line {
	limit, separators := len(children), 0
	for index, line := range children {
		if strings.Trim(line.Raw(), " \t") == "---" {
			limit, separators = index, separators+1
		}
	}
	if separators > 1 {
		return nil
	}
	items := make([]bool, len(children))
	n, k := 0, 0
	for index := 0; index < limit; index++ {
		filled, known := v179Item(children[index])
		if !known {
			return nil
		}
		items[index] = filled
		if filled {
			n++
		}
	}
	for index, line := range children {
		if v179Name(line) != "answer" {
			continue
		}
		if index >= limit || index == 0 || !items[index-1] {
			return nil
		}
		k++
	}
	if n < 2 || k == 0 || k == n {
		return nil
	}
	var missing []diagnostics.Line
	for index := 0; index < limit; index++ {
		if items[index] && (index+1 == limit || v179Name(children[index+1]) != "answer") {
			missing = append(missing, children[index])
		}
	}
	return missing
}

// v179Name читает tag; для ошибочной строки допускает точное начальное имя
// из Raw. Экранированный псевдотег и обычное содержимое не становятся тегами.
func v179Name(line diagnostics.Line) string {
	name, count := "", 0
	erroneous := line.Type() == "invalid"
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" {
			name, _ = element.Value()
			count++
		}
		erroneous = erroneous || element.Type() == "unparsed"
	}
	if count == 1 {
		return strings.ToLower(name)
	}
	if count != 0 || !erroneous {
		return ""
	}
	raw := []rune(strings.TrimLeft(line.Raw(), " \t"))
	if len(raw) == 0 || raw[0] != '@' {
		return ""
	}
	end := 1
	for end < len(raw) && !unicode.IsSpace(raw[end]) && raw[end] != '{' && raw[end] != '}' {
		end++
	}
	return strings.ToLower(string(raw[1:end]))
}

// v179Item определяет непустую обычную строку после синтаксической обработки.
// Второй результат означает определимость; одна строка всегда даёт один элемент.
func v179Item(line diagnostics.Line) (filled, known bool) {
	raw := strings.Trim(line.Raw(), " \t")
	if raw == "" {
		return false, true
	}
	switch line.Type() {
	case "tag", "block-start", "block-end", "heading":
		return false, true
	}
	if strings.HasPrefix(raw, "@") || strings.HasPrefix(raw, "#") {
		return false, true
	}
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" || element.Type() == "heading-level" {
			return false, true
		}
	}
	if line.Type() != "content" {
		return false, false
	}
	runes := []rune(line.Raw())
	cursor, contents := 0, 0
	complete := true
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if strings.Trim(string(runes[cursor:element.Start()-1]), " \t") != "" {
			complete = false
		}
		cursor = element.End() - 1
		if element.Type() != "content" {
			complete = false
			continue
		}
		contents++
		value, present := element.Value()
		complete = complete && present
		filled = filled || (present && value != "")
	}
	if filled {
		return true, true
	}
	if contents != 0 {
		return false, complete && strings.Trim(string(runes[cursor:]), " \t") == ""
	}
	if line.ElementCount() != 0 {
		return false, false
	}
	value, ok := v179Text(raw)
	return value != "" && ok, ok
}

// v179Text снимает экранирование исходной обычной строки ordering по 1.21–1.22.
// Готовые Value повторно не декодируются. false означает неясную сырую разметку.
func v179Text(raw string) (string, bool) {
	if raw == `\---` {
		return "---", true
	}
	runes := []rune(raw)
	var text strings.Builder
	for index := 0; index < len(runes); index++ {
		char := runes[index]
		if char == '\\' && index+1 < len(runes) {
			next := runes[index+1]
			if next == '\\' || next == '{' || next == '}' || (index == 0 && (next == '@' || next == '#')) {
				text.WriteRune(next)
				index++
				continue
			}
		}
		if char == '{' || char == '}' {
			return "", false
		}
		text.WriteRune(char)
	}
	return text.String(), true
}
