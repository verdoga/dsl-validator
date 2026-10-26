package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV137 регистрирует отсутствие непустых ответов только для DSL 1.2.
func registrationV137(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code: "V137", Severity: diagnostics.SeverityError, Scope: diagnostics.ScopeBlock,
		Message:  "Блок @answer не содержит непустых ответов.",
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV137,
	})
}

// checkV137 отмечает доказанно пустые блоки по готовым родительским связям.
// Диапазон начинается с @answer и заканчивается после закрывающей скобки;
// колонки Unicode с единицы, правая граница исключается.
func checkV137(lines diagnostics.Lines, findings diagnostics.Findings) error {
	index := v137Index(lines)
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if line.Type() != "block-start" {
			continue
		}
		tag := v137Element(line, "tag")
		if tag == nil {
			continue
		}
		name, _ := tag.Value()
		block := index[number]
		if name != "answer" || block.literal || !block.bounds || !block.empty || block.unknown ||
			index[block.end-1].unknownBlocks != block.unknownBlocks {
			continue
		}
		closing, _ := lines.Line(block.end)
		brace := v137Element(closing, "block-close")
		if _, err := findings.Add(number, tag.Start(), closing.Number(), brace.End(), ""); err != nil {
			return err
		}
	}
	return nil
}

// v137Element возвращает единственный элемент типа; nil означает неопределённость.
func v137Element(line diagnostics.Line, kind string) diagnostics.Element {
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

// v137LineInfo хранит сведения только текущего запуска, по физическому номеру.
// rootBlanks и unknownBlocks — префиксные счётчики с включённой текущей строкой.
// empty и unknown относятся к непосредственным строкам, bounds — к границам блока.
type v137LineInfo struct {
	end, descendants, lastDescendant int
	rootBlanks, unknownBlocks        int
	literal, bounds, empty, unknown  bool
}

// v137Index собирает сведения за O(N+E+R) времени; сам индекс занимает O(N) памяти:
// N — строки, E — элементы, R — длина исходного и обработанного текста. Родитель всегда
// предшествует потомку по CheckModel, поэтому обратный проход суммирует поддеревья
// без обхода цепочек родителей. Текст и границы каждой строки проверяются один раз.
func v137Index(lines diagnostics.Lines) []v137LineInfo {
	index := make([]v137LineInfo, lines.Len()+1)
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		info := &index[number]
		info.empty, info.lastDescendant = true, number
		info.rootBlanks = index[number-1].rootBlanks
		empty, known := v137EmptyLine(line)
		if parent, found := line.ParentLine(); found {
			info.literal = index[parent].literal
			if line.Type() == "block-end" {
				if index[parent].end == 0 {
					index[parent].end = number
				} else {
					index[parent].end = -1
				}
			} else {
				index[parent].empty = index[parent].empty && empty
				index[parent].unknown = index[parent].unknown || !known
			}
		} else if line.Type() == "blank" && empty && known && strings.Trim(line.Raw(), " \t") == "" {
			// Известная корневая blank не задаёт родителя и не разрывает тело.
			info.rootBlanks++
		}
		if line.Type() == "block-start" {
			if tag := v137Element(line, "tag"); tag != nil {
				name, _ := tag.Value()
				if name == "editor" || name == "multifill" {
					info.literal = true
				}
			}
		}
	}
	for number := lines.Len(); number > 0; number-- {
		line, _ := lines.Line(number)
		if parent, found := line.ParentLine(); found {
			index[parent].descendants += index[number].descendants + 1
			index[parent].lastDescendant = max(index[parent].lastDescendant, index[number].lastDescendant)
		}
	}
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		index[number].unknownBlocks = index[number-1].unknownBlocks
		if line.Type() == "block-start" {
			index[number].bounds = v137Bounds(lines, number, index)
			if !index[number].bounds {
				index[number].unknownBlocks++
			}
		}
	}
	return index
}

// v137Bounds подтверждает единственные скобки и непрерывное тело блока.
// false означает отсутствующие, неоднозначные либо противоречивые границы.
// Скобки из unparsed не восстанавливаются; параметры объявления не являются телом.
func v137Bounds(lines diagnostics.Lines, opening int, index []v137LineInfo) bool {
	block := index[opening]
	end := block.end
	if end <= opening {
		return false
	}
	// Последний потомок исключает связи за закрытием. Тогда равенство числа
	// потомков и известных корневых blank длине тела доказывает его непрерывность.
	if block.lastDescendant != end ||
		block.descendants+index[end].rootBlanks-block.rootBlanks != end-opening {
		return false
	}
	first, _ := lines.Line(opening)
	last, _ := lines.Line(end)
	left, right := v137Element(first, "block-open"), v137Element(last, "block-close")
	if left == nil || right == nil || strings.Trim(last.Raw(), " \t") != "}" {
		return false
	}
	if strings.Trim(string([]rune(first.Raw())[left.End()-1:]), " \t") != "" {
		return false
	}
	return true
}

// v137EmptyLine возвращает пустоту известных значений и полноту непосредственной
// строки отдельно. При known == false пустота всей строки не доказана;
// содержимое вложенных тегов не является значением ответа.
func v137EmptyLine(line diagnostics.Line) (empty, known bool) {
	switch line.Type() {
	case "tag", "block-start":
		return true, v137Element(line, "tag") != nil
	case "content", "blank":
		empty, known = true, v137TextKnown(line)
		for i := 0; i < line.ElementCount(); i++ {
			element, _ := line.Element(i)
			if element.Type() != "content" {
				continue
			}
			value, present := element.Value()
			known = known && present
			if present && answerPrimary(value) != "" {
				empty = false
			}
		}
		return empty, known
	default:
		return true, false
	}
}

// v137TextKnown проверяет полноту текстовых участков одной строки.
// Между content допустимы исходные U+0020/U+0009; unparsed и другие элементы
// делают значение неизвестным. Полностью пустая строка может не иметь элементов.
func v137TextKnown(line diagnostics.Line) bool {
	raw := []rune(line.Raw())
	end := 0
	for i := 0; i < line.ElementCount(); i++ {
		element, _ := line.Element(i)
		if element.Type() != "content" {
			return false
		}
		// Нулевой content может лежать внутри предыдущего участка. Он не
		// добавляет покрытия и не сдвигает уже известную правую границу.
		if element.Start() == element.End() {
			continue
		}
		if strings.Trim(string(raw[end:element.Start()-1]), " \t") != "" {
			return false
		}
		end = element.End() - 1
	}
	return strings.Trim(string(raw[end:]), " \t") == ""
}
