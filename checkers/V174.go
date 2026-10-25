package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// v174Group хранит число вопросов и исходные границы одной группы задания.
type v174Group struct {
	total, automatic int
	first, start     int
	last             int
}

// registrationV174 регистрирует смешанную автопроверку вопросов одного задания.
func registrationV174(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V174",
		Severity: diagnostics.SeverityError,
		Message:  "В одном задании только часть вопросов с выбором ответа имеет автопроверку.",
		Scope:    diagnostics.ScopeBlock,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV174,
	})
}

// checkV174 объединяет choice и multichoice по ближайшему task в готовой
// цепочке родителей. Неоднозначное закрытие последнего вопроса подавляет
// только сообщение его группы, не изменяя состав и подсчёт вопросов.
func checkV174(lines diagnostics.Lines, findings diagnostics.Findings) error {
	answers := make(map[int]bool)
	ends := make(map[int]int)
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		parent, found := line.ParentLine()
		if !found {
			continue
		}
		switch line.Type() {
		case "tag", "block-start":
			if tag := v174Element(line, "tag"); tag != nil && strings.EqualFold(tag.Raw(), "@answer") {
				answers[parent] = true
			}
		case "block-end":
			if _, exists := ends[parent]; exists {
				ends[parent] = -1
			} else {
				ends[parent] = number
			}
		}
	}
	groups := make(map[int]*v174Group)
	var order []int
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if line.Type() != "block-start" {
			continue
		}
		tag := v174Element(line, "tag")
		if tag == nil || !strings.EqualFold(tag.Raw(), "@choice") && !strings.EqualFold(tag.Raw(), "@multichoice") {
			continue
		}
		task := v174Task(lines, line)
		if task == 0 {
			continue
		}
		group := groups[task]
		if group == nil {
			group = &v174Group{first: number, start: tag.Start()}
			groups[task] = group
			order = append(order, task)
		}
		group.total++
		if answers[number] {
			group.automatic++
		}
		group.last = number
	}
	for _, task := range order {
		group := groups[task]
		if group.automatic == 0 || group.automatic == group.total {
			continue
		}
		end := ends[group.last]
		if end <= group.last {
			continue
		}
		closing, _ := lines.Line(end)
		brace := v174Element(closing, "block-close")
		if brace == nil {
			continue
		}
		if _, err := findings.Add(group.first, group.start, end, brace.End(), ""); err != nil {
			return err
		}
	}
	return nil
}

// v174Task возвращает номер ближайшего task-предка или ноль при его отсутствии.
func v174Task(lines diagnostics.Lines, line diagnostics.Line) int {
	for number, found := line.ParentLine(); found; number, found = line.ParentLine() {
		line, _ = lines.Line(number)
		if tag := v174Element(line, "tag"); tag != nil && strings.EqualFold(tag.Raw(), "@task") {
			return number
		}
	}
	return 0
}

// v174Element получает сохранённый элемент нужного типа без разбора Raw строки.
func v174Element(line diagnostics.Line, kind string) diagnostics.Element {
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == kind {
			return element
		}
	}
	return nil
}
