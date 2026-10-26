package checkers

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV156 регистрирует недостаток вариантов выбора только для DSL 1.2.
func registrationV156(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code: "V156", Severity: diagnostics.SeverityError, Scope: diagnostics.ScopeBlock,
		Message:  "В блоке выбора меньше двух вариантов.",
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV156,
	})
}

// checkV156 считает варианты по готовой структуре, не восстанавливая парсер.
// Диапазон блока начинается с тега и заканчивается после закрывающей скобки.
func checkV156(lines diagnostics.Lines, findings diagnostics.Findings) error {
	index := v156Index(lines)
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		tag := v156Element(line, "tag")
		if line.Type() != "block-start" || tag == nil {
			continue
		}
		name, _ := tag.Value()
		if (!strings.EqualFold(name, "choice") && !strings.EqualFold(name, "multichoice")) ||
			index[number].literal || index[number].uncertain {
			continue
		}
		count, known := v156Count(lines, number, index, false)
		if !known || count >= 2 {
			continue
		}
		closing, _ := lines.Line(index[number].end)
		brace := v156Element(closing, "block-close")
		if _, err := findings.Add(line.Number(), tag.Start(), closing.Number(), brace.End(),
			fmt.Sprintf("Количество вариантов: %d", count)); err != nil {
			return err
		}
	}
	return nil
}

// v156Element возвращает единственный элемент данного типа; nil означает неопределённость.
func v156Element(line diagnostics.Line, kind string) diagnostics.Element {
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

// v156Row хранит включающую границу, значимые строки поддерева и их префикс.
// end=-1 означает повторное закрытие; uncertain — неоднозначность своего
// либо вложенного блока. Пустые blank не влияют на размер и последнюю строку.
type v156Row struct {
	end, last, size, prefix int
	children                []int
	literal, uncertain      bool
}

// v156Index за два прохода готовит границы, детей и литеральный контекст.
// Prepare гарантирует предшествующего родителя. Поэтому обратный проход
// передаёт готовые итоги родителю без обхода цепочек предков и хвоста документа.
func v156Index(lines diagnostics.Lines) []v156Row {
	index := make([]v156Row, lines.Len()+1)
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		current := &index[number]
		current.last, current.size = number, 1
		if line.Type() == "blank" {
			if text, known := v156Text(line, 0, true); known && text == "" {
				current.last, current.size = 0, 0
			}
		}
		current.prefix = index[number-1].prefix + current.size
		if parent, found := line.ParentLine(); found {
			current.literal = index[parent].literal
			index[parent].children = append(index[parent].children, number)
			if line.Type() == "block-end" {
				if index[parent].end != 0 {
					index[parent].end = -1
				} else {
					index[parent].end = number
				}
			}
		}
		if line.Type() == "block-start" {
			if tag := v156Element(line, "tag"); tag != nil {
				name, _ := tag.Value()
				if strings.EqualFold(name, "editor") || strings.EqualFold(name, "multifill") {
					current.literal = true
				}
			}
		}
	}
	for number := lines.Len(); number > 0; number-- {
		line, _ := lines.Line(number)
		current := &index[number]
		if line.Type() == "block-start" && !v156Bounds(lines, number, index) {
			current.uncertain = true
		}
		if parent, found := line.ParentLine(); found {
			index[parent].last = max(index[parent].last, current.last)
			index[parent].size += current.size
			index[parent].uncertain = index[parent].uncertain || current.uncertain
		}
	}
	return index
}

// v156Bounds подтверждает единственное закрытие и непрерывность тела блока.
// Размер и максимум поддерева сравниваются с физическим интервалом без пустых
// blank: их корневой ParentLine допустим. false означает неизвестные либо
// противоречивые границы; скобки из Raw не восстанавливаются.
func v156Bounds(lines diagnostics.Lines, opening int, index []v156Row) bool {
	current := index[opening]
	end := current.end
	if end <= opening || current.last != end || current.size != index[end].prefix-index[opening-1].prefix {
		return false
	}
	first, _ := lines.Line(opening)
	last, _ := lines.Line(end)
	left, right := v156Element(first, "block-open"), v156Element(last, "block-close")
	if left == nil || right == nil || strings.Trim(last.Raw(), " \t") != "}" ||
		strings.Trim(string([]rune(first.Raw())[left.End()-1:]), " \t") != "" {
		return false
	}
	return true
}

// v156Count считает непосредственные варианты или строки блочного ответа.
// known=false запрещает вывод о недостатке; исключённый текст вариантов не создаёт.
func v156Count(lines diagnostics.Lines, opening int, index []v156Row, answer bool) (count int, known bool) {
	for _, number := range index[opening].children {
		if number == index[opening].end {
			continue
		}
		line, _ := lines.Line(number)
		if tag := v156Element(line, "tag"); tag != nil {
			name, _ := tag.Value()
			if answer || !strings.EqualFold(name, "answer") {
				continue
			}
			if line.Type() == "block-start" {
				nested, known := v156Count(lines, number, index, true)
				if !known {
					return 0, false
				}
				count += nested
			} else if line.Type() == "tag" {
				text, known := v156Text(line, tag.End()-1, false)
				if !known {
					return 0, false
				}
				if strings.TrimFunc(text, unicode.IsSpace) != "" {
					count++
				}
			} else {
				return 0, false
			}
			continue
		}
		if line.Type() != "content" && line.Type() != "blank" {
			return 0, false
		}
		text, known := v156Text(line, 0, true)
		if !known {
			return 0, false
		}
		if answer {
			text = strings.TrimFunc(text, unicode.IsSpace)
		}
		if text != "" {
			count++
		}
	}
	return count, true
}

// v156Text получает известный текст от исходного смещения start в рунах с нуля.
// Поддерживаются content и распознанные placeholder; непрочитанные участки дают false.
// Краевые U+0020/U+0009 удаляются до экранирования; initial разрешает начальные \@ и \#.
func v156Text(line diagnostics.Line, start int, initial bool) (text string, known bool) {
	raw := []rune(line.Raw())
	end := start
	for i := 0; i < line.ElementCount(); i++ {
		element, _ := line.Element(i)
		if element.End()-1 <= start {
			continue
		}
		if element.Start()-1 < end || (element.Type() != "content" && element.Type() != "placeholder") ||
			strings.Trim(string(raw[end:element.Start()-1]), " \t") != "" {
			return "", false
		}
		end = element.End() - 1
	}
	if strings.Trim(string(raw[end:]), " \t") != "" {
		return "", false
	}
	raw = []rune(strings.Trim(string(raw[start:]), " \t"))
	var decoded strings.Builder
	for i := 0; i < len(raw); i++ {
		if raw[i] == '\\' && i+1 < len(raw) &&
			(strings.ContainsRune("{}\\", raw[i+1]) || initial && i == 0 && strings.ContainsRune("@#", raw[i+1])) {
			i++
		}
		decoded.WriteRune(raw[i])
	}
	return decoded.String(), true
}
