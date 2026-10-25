package checkers

import (
	"strings"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV154 регистрирует рекомендацию поместить инструкцию в multifill.
func registrationV154(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V154",
		Severity: diagnostics.SeverityRecommendation,
		Message:  "@multifill без инструкции расположен сразу после однострочного @instr.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV154,
	})
}

// checkV154 проверяет строку объявления независимо от предзаполненного тела.
// Связанный диапазон охватывает всю исходную строку instr без перевода строки.
func checkV154(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		tag := v154WithoutInstruction(line)
		if tag == nil || v154Literal(lines, line) {
			continue
		}
		instruction := v154PreviousInstruction(lines, line)
		if instruction == nil {
			continue
		}
		occurrence, err := findings.Add(number, tag.Start(), number, tag.End(),
			"Если инструкция относится только к этому полю, перенесите её в строку объявления @multifill как INSTRUCTION. Если она общая для нескольких действий, оставьте отдельный @instr.")
		if err != nil {
			return err
		}
		if err := findings.AddRelated(occurrence, instruction.Number(), 1, instruction.Number(), utf8.RuneCountInString(instruction.Raw())+1); err != nil {
			return err
		}
	}
	return nil
}

// v154WithoutInstruction возвращает tag, если весь участок инструкции пуст
// или состоит из U+0020. Непустой unparsed не исчезает из исходного участка.
func v154WithoutInstruction(line diagnostics.Line) diagnostics.Element {
	if !v154Named(line, "multifill") {
		return nil
	}
	switch line.Type() {
	case "tag", "block-start", "invalid":
	default:
		return nil
	}
	tag := v154Element(line, "tag")
	raw := []rune(line.Raw())
	start, end := tag.End()-1, len(raw)
	if opening := v154Element(line, "block-open"); opening != nil {
		end = opening.Start() - 1
	}
	if end < start {
		return nil
	}
	for _, char := range raw[start:end] {
		if char != ' ' {
			return nil
		}
	}
	return tag
}

// v154PreviousInstruction ищет только ближайшую значимую конструкцию.
// nil означает границу, другую конструкцию либо instr без непустого content.
func v154PreviousInstruction(lines diagnostics.Lines, field diagnostics.Line) diagnostics.Line {
	parent, _ := field.ParentLine()
	for number := field.Number() - 1; number > parent; number-- {
		line, _ := lines.Line(number)
		if strings.Trim(line.Raw(), " \t") == "" {
			continue
		}
		if line.Type() == "block-end" {
			opening := v154BlockParent(lines, line)
			if opening == nil || !v154Comment(lines, opening, field) {
				return nil
			}
			number = opening.Number()
			continue
		}
		if !v154SameLevel(line, field) || !v154SingleLine(line) {
			return nil
		}
		if v154Comment(lines, line, field) {
			continue
		}
		if v154Named(line, "instr") {
			for index := 0; index < line.ElementCount(); index++ {
				element, _ := line.Element(index)
				value, present := element.Value()
				if element.Type() == "content" && present && strings.Trim(value, " \t") != "" {
					return line
				}
			}
		}
		return nil
	}
	return nil
}

// v154SingleLine возвращает true для сохранённого однострочного объявления,
// в том числе с ошибочным хвостом; фигурная граница всегда исключает строку.
func v154SingleLine(line diagnostics.Line) bool {
	return (line.Type() == "tag" || line.Type() == "invalid") &&
		v154Element(line, "block-open") == nil && v154Element(line, "block-close") == nil
}

// v154Comment разрешает пропуск editor того же контекста только там, где
// разрешено его размещение. Среди фигурных предков допустим только variants.
func v154Comment(lines diagnostics.Lines, line, field diagnostics.Line) bool {
	if !v154SameLevel(line, field) || !v154Named(line, "editor") {
		return false
	}
	for block := v154BlockParent(lines, line); block != nil; block = v154BlockParent(lines, block) {
		if !v154Named(block, "variants") {
			return false
		}
	}
	return true
}

// v154Literal возвращает true для литерального содержимого editor и multifill.
func v154Literal(lines diagnostics.Lines, line diagnostics.Line) bool {
	for block := v154BlockParent(lines, line); block != nil; block = v154BlockParent(lines, block) {
		if v154Named(block, "editor") || v154Named(block, "multifill") {
			return true
		}
	}
	return false
}

// v154BlockParent возвращает ближайшего фигурного предка по готовым связям.
func v154BlockParent(lines diagnostics.Lines, line diagnostics.Line) diagnostics.Line {
	for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
		line, _ = lines.Line(parent)
		if line.Type() == "block-start" || v154Element(line, "block-open") != nil {
			return line
		}
	}
	return nil
}

// v154SameLevel возвращает true при совпадении родителя и логической глубины.
func v154SameLevel(left, right diagnostics.Line) bool {
	leftParent, leftHasParent := left.ParentLine()
	rightParent, rightHasParent := right.ParentLine()
	return leftParent == rightParent && leftHasParent == rightHasParent && left.NestingLevel() == right.NestingLevel()
}

// v154Named сравнивает сохранённое имя тега без учёта регистра.
func v154Named(line diagnostics.Line, name string) bool {
	tag := v154Element(line, "tag")
	if tag == nil {
		return false
	}
	value, present := tag.Value()
	return present && strings.EqualFold(value, name)
}

// v154Element возвращает существующий элемент либо nil, не восстанавливая Raw.
func v154Element(line diagnostics.Line, kind string) diagnostics.Element {
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == kind {
			return element
		}
	}
	return nil
}
