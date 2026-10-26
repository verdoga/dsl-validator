package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV165 регистрирует недостаток нижних вариантов matching для DSL 1.2.
func registrationV165(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code: "V165", Severity: diagnostics.SeverityError, Scope: diagnostics.ScopeBlock,
		Message:  "В нижнем списке @matching меньше двух вариантов.",
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV165,
	})
}

// checkV165 считает непосредственные обычные строки нижней части. Неопределённое
// верхнее или исключённое вложенное содержимое само по себе подсчёт не отменяет.
func checkV165(lines diagnostics.Lines, findings diagnostics.Findings) error {
	index := v165Index(lines)
	for number := 1; number < len(index); number++ {
		block := &index[number]
		if block.line.Type() != "block-start" || v165Name(block.line) != "matching" || block.uncertain || block.literal {
			continue
		}
		separator, column, known := 0, 0, true
		for _, current := range block.children {
			line := index[current].line
			if current >= block.end || strings.Trim(line.Raw(), " \t") != "---" {
				continue
			}
			if separator != 0 {
				known = false
			}
			separator = current
			column = len(line.Raw()) - len(strings.TrimLeft(line.Raw(), " \t")) + 1
		}
		if !known || separator == 0 {
			continue
		}
		count := 0
		for _, current := range block.children {
			line := index[current].line
			if current <= separator || current >= block.end || line.Type() == "tag" || line.Type() == "block-start" {
				continue
			}
			nonempty, parsed := v165Content(line)
			if !parsed {
				known = false
				break
			}
			if nonempty {
				count++
			}
		}
		if known && count < 2 {
			closing := index[block.end].line
			brace := v165Element(closing, "block-close")
			if _, err := findings.Add(separator, column, closing.Number(), brace.End(), ""); err != nil {
				return err
			}
		}
	}
	return nil
}

// v165Name читает точное начальное имя уже распознанного объявления.
func v165Name(line diagnostics.Line) string {
	name := strings.TrimLeft(line.Raw(), " \t")
	if !strings.HasPrefix(name, "@") {
		return ""
	}
	if end := strings.IndexAny(name, " \t{"); end >= 0 {
		name = name[:end]
	}
	return strings.ToLower(name[1:])
}

// v165Element возвращает единственный элемент типа; nil означает неопределённость.
func v165Element(line diagnostics.Line, kind string) diagnostics.Element {
	var found diagnostics.Element
	for i := 0; i < line.ElementCount(); i++ {
		element, _ := line.Element(i)
		if element.Type() == kind {
			if found != nil {
				return nil
			}
			found = element
		}
	}
	return found
}

// v165Row хранит исходную строку, непосредственных детей и итоги её поддерева.
// end=-1 означает повторное закрытие; uncertain относится только к границам
// своего или вложенных блоков. Из size, last и prefix исключены известные blank.
type v165Row struct {
	line                    diagnostics.Line
	end, last, size, prefix int
	children                []int
	literal, uncertain      bool
}

// v165Index за два линейных прохода готовит границы, детей и контекст.
// Родитель предшествует ребёнку по контракту Prepare; обратный проход передаёт
// готовые итоги родителю без повторных обходов предков и хвоста документа.
func v165Index(lines diagnostics.Lines) []v165Row {
	index := make([]v165Row, lines.Len()+1)
	for number := 1; number < len(index); number++ {
		line, _ := lines.Line(number)
		current := &index[number]
		current.line, current.last, current.size = line, number, 1
		if line.Type() == "blank" {
			if nonempty, known := v165Content(line); known && !nonempty {
				current.last, current.size = 0, 0
			}
		}
		current.prefix = index[number-1].prefix + current.size
		if parent, found := line.ParentLine(); found {
			current.literal = index[parent].literal
			index[parent].children = append(index[parent].children, number)
			if line.Type() == "block-end" {
				if index[parent].end == 0 {
					index[parent].end = number
				} else {
					index[parent].end = -1
				}
			}
		}
		if line.Type() == "block-start" {
			name := v165Name(line)
			current.literal = current.literal || name == "editor" || name == "multifill"
		}
	}
	for number := len(index) - 1; number > 0; number-- {
		current := &index[number]
		if current.line.Type() == "block-start" && !v165Bounds(number, index) {
			current.uncertain = true
		}
		if parent, found := current.line.ParentLine(); found {
			index[parent].last = max(index[parent].last, current.last)
			index[parent].size += current.size
			index[parent].uncertain = index[parent].uncertain || current.uncertain
		}
	}
	return index
}

// v165Bounds подтверждает единственное закрытие и непрерывность значимого тела.
// Размер и последняя строка поддерева сравниваются с физическим интервалом;
// известные blank допустимы с любым родителем. false означает неясные границы.
func v165Bounds(opening int, index []v165Row) bool {
	current := &index[opening]
	end := current.end
	if end <= opening || current.last != end || current.size != index[end].prefix-index[opening-1].prefix {
		return false
	}
	first, last := current.line, index[end].line
	left, right := v165Element(first, "block-open"), v165Element(last, "block-close")
	if left == nil || right == nil || strings.Trim(last.Raw(), " \t") != "}" ||
		strings.Trim(string([]rune(first.Raw())[left.End()-1:]), " \t") != "" {
		return false
	}
	return true
}

// v165Content возвращает непустоту по готовым content.Value без преобразований.
// parsed=false означает неизвестное содержимое; несколько элементов дают одну строку.
func v165Content(line diagnostics.Line) (nonempty, parsed bool) {
	if line.Type() == "blank" && strings.Trim(line.Raw(), " \t") == "" {
		for i := 0; i < line.ElementCount(); i++ {
			element, _ := line.Element(i)
			if element.Type() == "unparsed" {
				return false, false
			}
		}
		return false, true
	}
	if line.Type() != "content" {
		return false, false
	}
	raw := []rune(line.Raw())
	end := 0
	for i := 0; i < line.ElementCount(); i++ {
		element, _ := line.Element(i)
		value, present := element.Value()
		if element.Type() != "content" || !present || element.Start()-1 < end ||
			strings.Trim(string(raw[end:element.Start()-1]), " \t") != "" {
			return false, false
		}
		nonempty = nonempty || value != ""
		end = element.End() - 1
	}
	return nonempty, strings.Trim(string(raw[end:]), " \t") == ""
}
