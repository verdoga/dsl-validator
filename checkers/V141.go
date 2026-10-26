package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV141 регистрирует разделение answer и hint одним тегом для DSL 1.2.
func registrationV141(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code: "V141", Severity: diagnostics.SeverityError, Scope: diagnostics.ScopeLine,
		Message:  "Между @answer и @hint расположен другой тег.",
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV141,
	})
}

// checkV141 проверяет ровно две следующие значимые конструкции каждого answer.
// Диапазоны охватывают имена тегов в исходных Unicode-колонках с единицы,
// правая граница исключается. Связанные теги идут в порядке: препятствие, answer.
func checkV141(lines diagnostics.Lines, findings diagnostics.Findings) error {
	index := v141Index(lines)
	for number := 1; number < len(index); number++ {
		answer := &index[number]
		if answer.name != "answer" || answer.literal {
			continue
		}
		end := number
		if answer.line.Type() == "block-start" {
			end = answer.end
			if end == 0 {
				continue
			}
		}
		middle := v141Next(index, answer.line, end+1)
		if middle == nil || middle.line.Type() != "tag" {
			continue
		}
		if middle.name == "" || middle.name == "answer" || middle.name == "hint" || v141Boundary(middle.name) {
			continue
		}
		hint := v141Next(index, answer.line, middle.line.Number()+1)
		if hint == nil || hint.name != "hint" {
			continue
		}
		occurrence, err := findings.Add(hint.line.Number(), hint.tag.Start(), hint.line.Number(), hint.tag.End(), "")
		if err != nil {
			return err
		}
		if err := findings.AddRelated(occurrence, middle.line.Number(), middle.tag.Start(), middle.line.Number(), middle.tag.End()); err != nil {
			return err
		}
		if err := findings.AddRelated(occurrence, number, answer.tag.Start(), number, answer.tag.End()); err != nil {
			return err
		}
	}
	return nil
}

// v141Element возвращает единственный элемент типа; nil означает неопределённость.
func v141Element(line diagnostics.Line, kind string) diagnostics.Element {
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

// v141Tag читает распознанное начальное объявление, не восстанавливая unparsed.
// Текстовые значения и экранированные упоминания повторно тегами не становятся.
func v141Tag(line diagnostics.Line) (string, diagnostics.Element) {
	if line.Type() != "tag" && line.Type() != "block-start" {
		return "", nil
	}
	tag := v141Element(line, "tag")
	start := len(line.Raw()) - len(strings.TrimLeft(line.Raw(), " \t"))
	if tag == nil || tag.Start() != start+1 || !strings.HasPrefix(line.Raw()[start:], "@") {
		return "", nil
	}
	name, _ := tag.Value()
	return strings.ToLower(name), tag
}

// v141Boundary перечисляет объявления, меняющие структурный контекст поиска.
func v141Boundary(name string) bool {
	switch name {
	case "task", "endtask", "header", "step", "newpage", "variants", "variant":
		return true
	}
	return false
}

// v141SameLevel сравнивает родителя и логическую глубину готовой модели.
func v141SameLevel(left, right diagnostics.Line) bool {
	lp, lf := left.ParentLine()
	rp, rf := right.ParentLine()
	return lp == rp && lf == rf && left.NestingLevel() == right.NestingLevel()
}

// v141Line хранит сведения одного запуска; end равен нулю при неясных границах.
// literal обозначает editor/multifill среди предков; comments разрешает editor
// вне фигурных контейнеров, кроме variants. Индекс не меняет исходную модель.
type v141Line struct {
	line              diagnostics.Line
	name              string
	tag               diagnostics.Element
	blank, uncertain  bool
	literal, comments bool
	end               int
}

// v141Index за два прохода собирает контексты и однозначные границы блоков.
// Родитель предшествует ребёнку по контракту Prepare. Обратный проход суммирует
// потомков без обходов цепочек предков; blank не влияет на непрерывность блока.
func v141Index(lines diagnostics.Lines) []v141Line {
	index := make([]v141Line, lines.Len()+1)
	ends, prefix := make([]int, len(index)), make([]int, len(index))
	// count и last учитывают только значимые строки поддерева, включая корень;
	// badOpening и maxEnd проверяют закрытия вложенных block-start.
	branches := make([]struct {
		parent, count, last, maxEnd int
		badOpening                  bool
	}, len(index))
	for number := 1; number < len(index); number++ {
		line, _ := lines.Line(number)
		item := &index[number]
		item.line, item.comments = line, true
		item.name, item.tag = v141Tag(line)
		item.uncertain = line.Type() == "invalid"
		for i := 0; i < line.ElementCount(); i++ {
			element, _ := line.Element(i)
			item.uncertain = item.uncertain || element.Type() == "unparsed"
		}
		item.blank = line.Type() == "blank" && !item.uncertain && strings.Trim(line.Raw(), " \t") == ""
		prefix[number] = prefix[number-1]
		if !item.blank {
			prefix[number]++
			branches[number].count, branches[number].last = 1, number
		}
		if parent, found := line.ParentLine(); found {
			branches[number].parent = parent
			ancestor := &index[parent]
			item.literal, item.comments = ancestor.literal, ancestor.comments
			if ancestor.line.Type() == "block-start" {
				item.literal = item.literal || ancestor.name == "editor" || ancestor.name == "multifill"
				item.comments = item.comments && ancestor.name == "variants"
			}
			if line.Type() == "block-end" {
				if ends[parent] == 0 {
					ends[parent] = number
				} else {
					ends[parent] = -1
				}
			}
		}
	}
	for number := len(index) - 1; number > 0; number-- {
		item, branch := &index[number], &branches[number]
		if item.line.Type() == "block-start" {
			end := ends[number]
			// Последний потомок на закрытии исключает связи за блоком; равенство
			// количеств значимых строк исключает чужие строки внутри его границ.
			if end > number && branch.last == end && branch.count-1 == prefix[end]-prefix[number] &&
				!branch.badOpening && branch.maxEnd < end && v141Element(item.line, "block-open") != nil &&
				v141Element(index[end].line, "block-close") != nil && strings.Trim(index[end].line.Raw(), " \t") == "}" {
				item.end = end
			}
			branch.badOpening = branch.badOpening || end <= number
			branch.maxEnd = max(branch.maxEnd, end)
		}
		if branch.parent != 0 {
			parent := &branches[branch.parent]
			parent.count += branch.count
			parent.last = max(parent.last, branch.last)
			parent.maxEnd = max(parent.maxEnd, branch.maxEnd)
			parent.badOpening = parent.badOpening || branch.badOpening
		}
	}
	return index
}

// v141Next возвращает ближайшую значимую конструкцию того же уровня либо nil.
// Неизвестная строка и смена контекста останавливают поиск. Пропускаются только
// пустые строки и разрешённые editor; произвольные блоки здесь не обходятся.
func v141Next(index []v141Line, answer diagnostics.Line, first int) *v141Line {
	for number := first; number < len(index); number++ {
		item := &index[number]
		if item.blank {
			continue
		}
		if !v141SameLevel(answer, item.line) || item.uncertain {
			return nil
		}
		if item.name != "editor" || !item.comments {
			return item
		}
		if item.line.Type() == "block-start" {
			number = item.end
			if number == 0 {
				return nil
			}
		}
	}
	return nil
}
