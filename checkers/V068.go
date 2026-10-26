package checkers

import (
	"strconv"
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// v068Task хранит число этапов и исходный диапазон имени открывающего тега.
// Колонки отсчитываются с единицы в Unicode-символах; конец исключается.
type v068Task struct {
	line, start, end, steps int
}

// registrationV068 регистрирует минимум этапов для заданий DSL 1.2 с @step.
func registrationV068(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V068",
		Severity: diagnostics.SeverityError,
		Message:  "В задании меньше двух внутренних этапов @step.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV068,
	})
}

// checkV068 считает структурные этапы каждой области задания независимо.
// Задания без этапов не подлежат проверке минимального числа @step.
// Физические переходы между variant учитываются и при общем родителе
// variants у объявления ветви и её заданий. Включения не раскрываются.
func checkV068(lines diagnostics.Lines, findings diagnostics.Findings) error {
	var task v068Task
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		variants, structural := v068Context(lines, line)
		if !structural {
			continue
		}
		tag := v068Tag(line)
		if v068Boundary(line, tag, variants) {
			if err := v068Report(task, findings); err != nil {
				return err
			}
			task = v068Task{}
		}
		if tag == "task" {
			element := v068Element(line, "tag")
			task = v068Task{line: number, start: element.Start(), end: element.End()}
		} else if task.line != 0 && v068Step(line, tag) {
			task.steps++
		}
	}
	return v068Report(task, findings)
}

// v068Report выдаёт одно сообщение на имя @task только при одном этапе.
// Нулевой номер строки означает отсутствие открытого задания.
func v068Report(task v068Task, findings diagnostics.Findings) error {
	if task.line == 0 || task.steps != 1 {
		return nil
	}
	_, err := findings.Add(task.line, task.start, task.line, task.end,
		"Найдено этапов: "+strconv.Itoa(task.steps)+".")
	return err
}

// v068Context возвращает ближайший variants и доступность структурных тегов.
// Любой другой фигурный предок исключает строку; его закрытие тоже не является
// границей задания. Отступы не используются для восстановления вложенности.
func v068Context(lines diagnostics.Lines, line diagnostics.Line) (variants int, structural bool) {
	for {
		parent, found := line.ParentLine()
		if !found {
			return variants, true
		}
		line, _ = lines.Line(parent)
		if line.Type() != "block-start" {
			continue
		}
		if v068Tag(line) != "variants" {
			return 0, false
		}
		if variants == 0 {
			variants = parent
		}
	}
}

// v068Boundary определяет завершение области по распознанной конструкции.
// true означает, что эта строка уже не принадлежит текущему заданию.
func v068Boundary(line diagnostics.Line, tag string, variants int) bool {
	if line.Type() == "block-end" {
		parent, _ := line.ParentLine()
		return variants != 0 && parent == variants
	}
	if line.Type() == "heading" {
		if level := v068Element(line, "heading-level"); level != nil {
			value, _ := level.Value()
			return value == "3"
		}
	}
	switch tag {
	case "task", "endtask", "header", "newpage", "variants":
		return true
	case "variant":
		return variants != 0
	}
	return false
}

// v068Step учитывает распознанное имя, даже если название отсутствует.
// Без tag восстанавливает только ошибочную строку по уточнению A:
// начальные U+0020/U+0009, точное @step, затем конец или U+0020/U+0009.
// true означает одно объявление; Raw не декодируется и не нормализуется.
func v068Step(line diagnostics.Line, tag string) bool {
	if tag != "" {
		return tag == "step"
	}
	erroneous := line.Type() == "invalid"
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" {
			return false
		}
		erroneous = erroneous || element.Type() == "unparsed"
	}
	if !erroneous {
		return false
	}
	raw := strings.TrimLeft(line.Raw(), " \t")
	return len(raw) >= 5 && strings.ToLower(raw[:5]) == "@step" &&
		(len(raw) == 5 || raw[5] == ' ' || raw[5] == '\t')
}

// v068Tag читает каноническое имя объявления, сохранившегося при разборе.
// invalid может содержать достоверный tag вместе с неразобранным остатком.
func v068Tag(line diagnostics.Line) string {
	switch line.Type() {
	case "tag", "block-start", "invalid":
		if element := v068Element(line, "tag"); element != nil {
			value, _ := element.Value()
			return strings.ToLower(value)
		}
	}
	return ""
}

// v068Element возвращает единственный элемент типа либо nil, если его нет
// или элементов несколько. Исходник не разбирается повторно.
func v068Element(line diagnostics.Line, kind string) diagnostics.Element {
	var result diagnostics.Element
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == kind {
			if result != nil {
				return nil
			}
			result = element
		}
	}
	return result
}
