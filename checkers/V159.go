package checkers

import (
	"strings"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV159 регистрирует проверку соседства подсказок выбора для DSL 1.2.
func registrationV159(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code: "V159", Severity: diagnostics.SeverityError, Scope: diagnostics.ScopeLine,
		Message:  "Подсказка @hint не следует непосредственно за @answer.",
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV159,
	})
}

// v159Construct хранит значимую конструкцию и исходный диапазон в Unicode-колонках
// с единицы; правая граница исключается. Блок представлен целиком.
type v159Construct struct {
	name                    string
	first, start, last, end int
}

// checkV159 проверяет не более двух предшествующих конструкций каждой подсказки.
// Связанные диапазоны идут в исходном порядке: ответ, промежуточная конструкция.
func checkV159(lines diagnostics.Lines, findings diagnostics.Findings) error {
	index := v159Index(lines)
	for number := 1; number <= lines.Len(); number++ {
		hint, _ := lines.Line(number)
		info := index[number]
		if info.name != "hint" || info.parent == 0 {
			continue
		}
		container := index[info.parent]
		if !container.block || (container.name != "choice" && container.name != "multichoice") || info.literal {
			continue
		}
		middle := v159Previous(lines, hint, number, index)
		if middle.name == "answer" {
			continue
		}
		detail := ""
		var answer v159Construct
		if middle.first != 0 {
			answer = v159Previous(lines, hint, middle.first, index)
			if answer.name == "answer" {
				if middle.name == "hint" {
					detail = "Повторная подсказка для одного @answer."
				} else {
					detail = "Промежуточная конструкция разрывает соседство."
				}
			}
		}
		occurrence, err := findings.Add(number, info.tag.Start(), number, info.tag.End(), detail)
		if err != nil {
			return err
		}
		if detail != "" {
			for _, related := range []v159Construct{answer, middle} {
				if err := findings.AddRelated(occurrence, related.first, related.start, related.last, related.end); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// v159Element возвращает единственный элемент типа либо nil при неоднозначности.
func v159Element(line diagnostics.Line, kind string) diagnostics.Element {
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

// v159Tag читает распознанный начальный тег; литеральный текст тегом не становится.
func v159Tag(line diagnostics.Line) (string, diagnostics.Element) {
	if line.Type() != "tag" && line.Type() != "block-start" {
		return "", nil
	}
	tag := v159Element(line, "tag")
	start := len(line.Raw()) - len(strings.TrimLeft(line.Raw(), " \t"))
	if tag == nil || tag.Start() != start+1 || !strings.HasPrefix(line.Raw()[start:], "@") {
		return "", nil
	}
	name, _ := tag.Value()
	return strings.ToLower(name), tag
}

// v159LineInfo хранит частный индекс текущего запуска, по физическому номеру.
// previous — строка перед конструкцией после пропуска известных root blank и
// пустых строк того же родителя; чужая структура остаётся явной остановкой.
// rootBlanks — префикс с включённой текущей строкой; незакрытые блоки учитываются
// в unclosedNested только у предков, без самого проверяемого блока.
type v159LineInfo struct {
	name                           string
	tag                            diagnostics.Element
	parent, previous, end          int
	descendants, lastDescendant    int
	rootBlanks                     int
	block, blank, unknown, literal bool
	bounds, unclosedNested         bool
}

// v159Index собирает сведения за O(N+E+R) времени; индекс занимает O(N) памяти.
// N — строки, E — элементы, R — суммарная длина исходного текста.
// CheckModel гарантирует предшествующего родителя: прямой проход наследует
// контекст, обратный суммирует поддеревья без обхода родительских цепочек.
func v159Index(lines diagnostics.Lines) []v159LineInfo {
	index := make([]v159LineInfo, lines.Len()+1)
	lastNonRootBlank := 0
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		info := &index[number]
		info.name, info.tag = v159Tag(line)
		info.parent, _ = line.ParentLine()
		info.block = line.Type() == "block-start"
		info.lastDescendant = number
		info.unknown = line.Type() == "invalid"
		for i := 0; i < line.ElementCount(); i++ {
			element, _ := line.Element(i)
			if element.Type() == "unparsed" {
				info.unknown = true
			}
		}
		info.blank = !info.unknown && line.Type() != "block-end" && strings.Trim(line.Raw(), " \t") == ""
		info.previous = lastNonRootBlank
		if previous := index[lastNonRootBlank]; previous.blank && previous.parent == info.parent {
			// Одинаковый родитель означает одинаковую глубину после CheckModel.
			info.previous = previous.previous
		}
		info.rootBlanks = index[number-1].rootBlanks
		if info.parent == 0 && line.Type() == "blank" && info.blank {
			info.rootBlanks++
		} else {
			lastNonRootBlank = number
		}
		info.literal = index[info.parent].literal || info.block && (info.name == "editor" || info.name == "multifill")
		if info.parent != 0 && line.Type() == "block-end" {
			if index[info.parent].end == 0 {
				index[info.parent].end = number
			} else {
				index[info.parent].end = -1
			}
		}
	}
	for number := lines.Len(); number > 0; number-- {
		info := index[number]
		if info.parent != 0 {
			parent := &index[info.parent]
			parent.descendants += info.descendants + 1
			parent.lastDescendant = max(parent.lastDescendant, info.lastDescendant)
			parent.unclosedNested = parent.unclosedNested || info.unclosedNested || info.block && info.end <= number
		}
	}
	for number := 1; number <= lines.Len(); number++ {
		if index[number].block {
			index[number].bounds = v159Bounds(lines, number, index)
		}
	}
	return index
}

// v159SameLevel сравнивает непосредственного родителя и глубину готовой модели.
func v159SameLevel(left, right diagnostics.Line) bool {
	lp, lf := left.ParentLine()
	rp, rf := right.ParentLine()
	return lp == rp && lf == rf && left.NestingLevel() == right.NestingLevel()
}

// v159Previous возвращает одну предыдущую конструкцию; нулевой first означает
// границу или неизвестное содержимое. Пропускаются только пустые строки.
func v159Previous(lines diagnostics.Lines, hint diagnostics.Line, number int, index []v159LineInfo) v159Construct {
	number = index[number].previous
	if number == 0 {
		return v159Construct{}
	}
	line, _ := lines.Line(number)
	info := index[number]
	if line.Type() == "block-end" {
		if info.parent == 0 {
			return v159Construct{}
		}
		opening, _ := lines.Line(info.parent)
		block := index[info.parent]
		if !v159SameLevel(hint, opening) || (block.name != "answer" && block.name != "hint") ||
			block.end != number || !block.bounds {
			return v159Construct{}
		}
		brace := v159Element(line, "block-close")
		return v159Construct{block.name, info.parent, block.tag.Start(), number, brace.End()}
	}
	if !v159SameLevel(hint, line) || info.unknown {
		return v159Construct{}
	}
	if line.Type() == "content" {
		return v159Construct{"", number, 1, number, utf8.RuneCountInString(line.Raw()) + 1}
	}
	if line.Type() != "tag" || info.name == "" {
		return v159Construct{}
	}
	switch info.name {
	case "task", "endtask", "header", "step", "newpage", "variants", "variant":
		return v159Construct{}
	}
	return v159Construct{info.name, number, info.tag.Start(), number, info.tag.End()}
}

// v159Bounds подтверждает однозначное закрытие и непрерывность тела блока.
// false запрещает переход через границу; Raw не используется для построения структуры.
func v159Bounds(lines diagnostics.Lines, opening int, index []v159LineInfo) bool {
	block := index[opening]
	end := block.end
	// Все потомки должны завершиться на end и вместе с известными root blank
	// покрыть весь интервал. Единственные закрытия вложенных блоков тогда лежат
	// строго внутри: каждое связано со своим открытием, а не с внешним блоком.
	if end <= opening || block.lastDescendant != end || block.unclosedNested ||
		block.descendants+index[end].rootBlanks-block.rootBlanks != end-opening {
		return false
	}
	first, _ := lines.Line(opening)
	last, _ := lines.Line(end)
	if v159Element(first, "block-open") == nil || v159Element(last, "block-close") == nil ||
		strings.Trim(last.Raw(), " \t") != "}" {
		return false
	}
	return true
}
