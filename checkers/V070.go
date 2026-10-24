package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV070 регистрирует проверку активности задания только для DSL 1.2.
func registrationV070(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V070",
		Severity: diagnostics.SeverityError,
		Message:  "@speaking находится вне активного задания.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV070,
	})
}

// checkV070 восстанавливает активность задания в порядке исходных строк.
// Ошибочные аргументы не отменяют распознанные имена. Состояние не переносится
// между ветвями, за закрытие variants или в следующий запуск диагностики.
func checkV070(lines diagnostics.Lines, findings diagnostics.Findings) error {
	active := false
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		variants, excluded := v070Context(lines, line)
		if excluded {
			continue
		}
		if line.Type() == "block-end" {
			if parent, found := line.ParentLine(); found && parent == variants {
				active = false
			}
			continue
		}
		if level := v070Element(line, "heading-level"); level != nil && level.Raw() == "###" {
			active = false
		}
		tag := v070Element(line, "tag")
		if tag == nil {
			continue
		}
		switch strings.ToLower(tag.Raw()) {
		case "@task":
			active = true
		case "@endtask", "@header", "@newpage", "@variants":
			active = false
		case "@variant":
			if variants != 0 {
				active = false
			}
		case "@speaking":
			if !active {
				// Scope line допускает диапазон только имени тега.
				// Start/End уже заданы в исходных Unicode-колонках [start,end).
				if _, err := findings.Add(number, tag.Start(), number, tag.End(), ""); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// v070Context возвращает номер ближайшего фигурного variants либо 0.
// excluded=true означает наличие иного фигурного предка: такие объявления
// не проверяются и не меняют внешнее задание. Логические task/step/variant
// сами по себе не являются исключёнными контейнерами.
func v070Context(lines diagnostics.Lines, line diagnostics.Line) (variants int, excluded bool) {
	for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
		line, _ = lines.Line(parent)
		if line.Type() != "block-start" {
			continue
		}
		tag := v070Element(line, "tag")
		if tag == nil || !strings.EqualFold(tag.Raw(), "@variants") {
			return 0, true
		}
		if variants == 0 {
			variants = parent
		}
	}
	return variants, false
}

// v070Element возвращает распознанный элемент либо nil, не разбирая строку заново.
func v070Element(line diagnostics.Line, kind string) diagnostics.Element {
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == kind {
			return element
		}
	}
	return nil
}
