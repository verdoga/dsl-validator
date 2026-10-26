package checkers

import (
	"strings"
	"unicode"

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
		if name != "answer" || block.literal || !block.bounds || !block.empty ||
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
// empty относится только к непосредственным строкам, bounds — к границам блока.
type v137LineInfo struct {
	end, descendants, lastDescendant int
	rootBlanks, unknownBlocks        int
	literal, bounds, empty           bool
}

// v137Index собирает сведения за O(N+E+R) времени; сам индекс занимает O(N) памяти:
// N — строки, E — элементы, R — суммарная длина исходного текста. Родитель всегда
// предшествует потомку по CheckModel, поэтому обратный проход суммирует поддеревья
// без обхода цепочек родителей. Текст и границы каждой строки проверяются один раз.
func v137Index(lines diagnostics.Lines) []v137LineInfo {
	index := make([]v137LineInfo, lines.Len()+1)
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		info := &index[number]
		info.empty, info.lastDescendant = true, number
		info.rootBlanks = index[number-1].rootBlanks
		if parent, found := line.ParentLine(); found {
			info.literal = index[parent].literal
			if line.Type() == "block-end" {
				if index[parent].end == 0 {
					index[parent].end = number
				} else {
					index[parent].end = -1
				}
			} else if !v137EmptyLine(line) {
				index[parent].empty = false
			}
		} else if line.Type() == "blank" && v137EmptyLine(line) {
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

// v137EmptyLine возвращает true при доказанной пустоте непосредственной строки.
// Неизвестный текст не заменяется пустотой, содержимое вложенных тегов исключено.
func v137EmptyLine(line diagnostics.Line) bool {
	switch line.Type() {
	case "tag", "block-start":
		return v137Element(line, "tag") != nil
	case "content", "blank":
		return v137TextKnown(line) && v137Text(line.Raw()) == ""
	default:
		return false
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

// v137Text получает отдельное значение строки answer по DSL 1.14, 1.21–1.22.
// После экранирования сжимается Unicode White_Space и удаляются края.
// Пунктуация, регистр и неизвестные escape-последовательности сохраняются;
// HTML и плейсхолдеры в ответах не интерпретируются.
func v137Text(source string) string {
	raw := []rune(strings.Trim(source, " \t"))
	var decoded strings.Builder
	for i := 0; i < len(raw); i++ {
		if raw[i] == '\\' && i+1 < len(raw) &&
			(strings.ContainsRune("{}\\", raw[i+1]) || i == 0 && strings.ContainsRune("@#", raw[i+1])) {
			i++
		}
		decoded.WriteRune(raw[i])
	}
	return strings.Join(strings.FieldsFunc(decoded.String(), func(char rune) bool {
		return unicode.Is(unicode.White_Space, char)
	}), " ")
}
