package checkers

import "github.com/verdoga/dsl-validator/diagnostics"

// v077Position хранит диапазон первого @hint с исключающей правой границей.
type v077Position struct{ line, start, end int }

// registrationV077 регистрирует ограничение числа подсказок инструкции для DSL 1.2.
func registrationV077(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V077",
		Severity: diagnostics.SeverityError,
		Message:  "В инструкции объявлено несколько подсказок @hint.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV077,
	})
}

// checkV077 считает непосредственные объявления hint отдельно для каждого instr.
// Содержимое подсказки не участвует в счёте; каждый повтор связан с первым именем.
func checkV077(lines diagnostics.Lines, findings diagnostics.Findings) error {
	first := make(map[int]v077Position)
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		tag := v077Tag(line, "hint")
		if tag == nil {
			continue
		}
		instruction := v077Instruction(lines, line)
		if instruction == 0 {
			continue
		}
		previous, found := first[instruction]
		if !found {
			first[instruction] = v077Position{number, tag.Start(), tag.End()}
			continue
		}
		occurrence, err := findings.Add(number, tag.Start(), number, tag.End(), "")
		if err != nil {
			return err
		}
		if err := findings.AddRelated(occurrence, previous.line, previous.start, previous.line, previous.end); err != nil {
			return err
		}
	}
	return nil
}

// v077Tag возвращает распознанный тег по каноническому имени либо nil.
func v077Tag(line diagnostics.Line, name string) diagnostics.Element {
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		value, _ := element.Value()
		if element.Type() == "tag" && value == name {
			return element
		}
	}
	return nil
}

// v077Instruction возвращает номер ближайшего фигурного родителя, если это instr.
// Другой фигурный родитель или отсутствие такого родителя дают 0.
func v077Instruction(lines diagnostics.Lines, line diagnostics.Line) int {
	for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
		line, _ = lines.Line(parent)
		if line.Type() == "block-start" {
			if v077Tag(line, "instr") != nil {
				return parent
			}
			return 0
		}
	}
	return 0
}
