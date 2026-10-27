package checkers

import (
	"strings"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV200 регистрирует редакторский порядок описаний только для DSL 1.2.
func registrationV200(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code: "V200", Severity: diagnostics.SeverityWarning, Scope: diagnostics.ScopeBlock,
		Message:  "Описание изображения расположено между инструкцией и образцом выполнения.",
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV200,
	})
}

// v200Range хранит исходный диапазон с исключающей правой границей.
// Нулевая first обозначает отсутствие конструкции.
type v200Range struct {
	first, start, last, end int
}

// checkV200 ищет значимых соседей в одном задании или его внутреннем этапе.
// Содержимое блоков не образует самостоятельных соседей; тексты не сравниваются.
func checkV200(lines diagnostics.Lines, findings diagnostics.Findings) error {
	ends := v200BlockEnds(lines)
	var task, owner diagnostics.Line
	var instruction, descriptions v200Range
	container := 0
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		current, allowed := v200Context(lines, line)
		if !allowed {
			instruction, descriptions = v200Range{}, v200Range{}
			continue
		}
		if current != container {
			task, owner, container = nil, nil, current
			instruction, descriptions = v200Range{}, v200Range{}
		}
		name := v200Tag(line)
		if line.Type() == "heading" || line.Type() == "block-end" {
			task, owner = nil, nil
			instruction, descriptions = v200Range{}, v200Range{}
			continue
		}
		switch name {
		case "task":
			task, owner = line, line
			instruction, descriptions = v200Range{}, v200Range{}
			continue
		case "step":
			owner = nil
			if task != nil && v200ChildOf(line, task) {
				owner = line
			}
			instruction, descriptions = v200Range{}, v200Range{}
			continue
		case "endtask", "header", "newpage", "variants", "variant":
			task, owner = nil, nil
			instruction, descriptions = v200Range{}, v200Range{}
			continue
		}
		if line.Type() == "blank" {
			continue
		}
		span, complete := v200Extent(lines, line, ends)
		// Любой известный блок проходит как единица, в том числе вне задания.
		// variants обрабатывается выше: его ветви содержат отдельные задания.
		if line.Type() == "block-start" && ends[number] > number {
			number = ends[number]
		}
		if owner == nil || !v200ChildOf(line, owner) || !complete {
			instruction, descriptions = v200Range{}, v200Range{}
			continue
		}
		switch name {
		case "editor":
			// Непосредственный editor разрешён в задании и внутреннем этапе.
			continue
		case "instr":
			instruction, descriptions = span, v200Range{}
		case "alt":
			if instruction.first != 0 {
				if descriptions.first == 0 {
					descriptions = span
				} else {
					descriptions.last, descriptions.end = span.last, span.end
				}
			}
		case "example":
			if instruction.first != 0 && descriptions.first != 0 {
				if err := v200Report(findings, descriptions, instruction, span); err != nil {
					return err
				}
			}
			instruction, descriptions = v200Range{}, v200Range{}
		default:
			instruction, descriptions = v200Range{}, v200Range{}
		}
	}
	return nil
}

// v200BlockEnds использует готовые закрытия и родительские связи модели.
// Повторное закрытие не задаёт однозначного полного диапазона.
func v200BlockEnds(lines diagnostics.Lines) map[int]int {
	ends := make(map[int]int)
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if line.Type() != "block-end" || v200Element(line, "block-close") == nil {
			continue
		}
		parent, found := line.ParentLine()
		if !found {
			continue
		}
		if _, exists := ends[parent]; exists {
			ends[parent] = 0
		} else {
			ends[parent] = number
		}
	}
	return ends
}

// v200Context возвращает ближайший variants либо 0 на уровне шага.
// allowed=false исключает содержимое иных фигурных контейнеров, включая
// литеральные editor/multifill. Объявления внутри них не меняют активное задание.
func v200Context(lines diagnostics.Lines, line diagnostics.Line) (container int, allowed bool) {
	for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
		line, _ = lines.Line(parent)
		if line.Type() != "block-start" {
			continue
		}
		if v200Tag(line) != "variants" {
			return 0, false
		}
		if container == 0 {
			container = parent
		}
	}
	return container, true
}

// v200ChildOf проверяет непосредственную принадлежность и логическую глубину.
func v200ChildOf(line, owner diagnostics.Line) bool {
	parent, found := line.ParentLine()
	return found && parent == owner.Number() && line.NestingLevel() == owner.NestingLevel()+1
}

// v200Extent возвращает диапазон от тега до конца однострочной конструкции
// без EOL либо до конца закрывающей скобки блока. false означает, что полный
// диапазон распознанной конструкции не установлен; Raw не разбирается заново.
func v200Extent(lines diagnostics.Lines, line diagnostics.Line, ends map[int]int) (v200Range, bool) {
	tag := v200Element(line, "tag")
	if v200Tag(line) == "" || tag == nil {
		return v200Range{}, false
	}
	span := v200Range{line.Number(), tag.Start(), line.Number(), utf8.RuneCountInString(line.Raw()) + 1}
	if line.Type() == "block-start" {
		end := ends[line.Number()]
		if end <= line.Number() {
			return v200Range{}, false
		}
		closing, _ := lines.Line(end)
		brace := v200Element(closing, "block-close")
		if brace == nil || !v200ChildOf(closing, line) {
			return v200Range{}, false
		}
		span.last, span.end = end, brace.End()
	}
	return span, true
}

// v200Report выдаёт одно сообщение на группу и два полных связанных диапазона.
func v200Report(findings diagnostics.Findings, group, instruction, example v200Range) error {
	occurrence, err := findings.Add(group.first, group.start, group.last, group.end,
		"Разместите эту группу описаний после образца выполнения, сохранив порядок описаний.")
	if err != nil {
		return err
	}
	for _, related := range [...]v200Range{instruction, example} {
		if err := findings.AddRelated(occurrence, related.first, related.start, related.last, related.end); err != nil {
			return err
		}
	}
	return nil
}

// v200Tag читает только распознанное имя; content и invalid не становятся тегами.
func v200Tag(line diagnostics.Line) string {
	if line.Type() != "tag" && line.Type() != "block-start" {
		return ""
	}
	if tag := v200Element(line, "tag"); tag != nil {
		value, _ := tag.Value()
		return strings.ToLower(value)
	}
	return ""
}

func v200Element(line diagnostics.Line, kind string) diagnostics.Element {
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == kind {
			return element
		}
	}
	return nil
}
