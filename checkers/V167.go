package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV167 регистрирует проверку соседства ответа и верхнего элемента matching.
func registrationV167(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code: "V167", Severity: diagnostics.SeverityError,
		Message:  "Ответ @answer не следует непосредственно за элементом верхнего списка.",
		Scope:    diagnostics.ScopeElement,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV167,
	})
}

// checkV167 проверяет непосредственные ответы только до единственного разделителя.
func checkV167(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		block, _ := lines.Line(number)
		name, _ := v167Tag(block)
		if block.Type() != "block-start" || name != "matching" || v167LiteralParent(lines, block) {
			continue
		}
		separator, count := 0, 0
		for next := number + 1; next <= lines.Len(); next++ {
			line, _ := lines.Line(next)
			parent, _ := line.ParentLine()
			if line.NestingLevel() <= block.NestingLevel() || parent == number && line.Type() == "block-end" {
				break
			}
			if parent == number && strings.Trim(line.Raw(), " \t") == "---" {
				separator, count = next, count+1
			}
		}
		if count != 1 {
			continue
		}
		for next := number + 1; next < separator; next++ {
			line, _ := lines.Line(next)
			parent, _ := line.ParentLine()
			name, tag := v167Tag(line)
			if parent != number || name != "answer" {
				continue
			}
			previous := v167Previous(lines, block, next)
			if previous != nil && previous.Type() == "content" && strings.Trim(previous.Raw(), " \t") != "" {
				continue
			}
			detail := ""
			if previous != nil {
				previousName, _ := v167Tag(previous)
				if previousName == "hint" {
					previous = v167Previous(lines, block, previous.Number())
					previousName = ""
					if previous != nil {
						previousName, _ = v167Tag(previous)
					}
				}
				if previousName == "answer" {
					detail = "Повторный @answer для элемента верхнего списка."
				}
			}
			if _, err := findings.Add(next, tag.Start(), next, tag.End(), detail); err != nil {
				return err
			}
		}
	}
	return nil
}

// v167Previous возвращает одну предыдущую значимую конструкцию своего блока.
// nil означает границу; содержимое завершённых answer и hint не просматривается.
func v167Previous(lines diagnostics.Lines, block diagnostics.Line, before int) diagnostics.Line {
	for number := before - 1; number > block.Number(); number-- {
		line, _ := lines.Line(number)
		parent, _ := line.ParentLine()
		if line.Type() == "block-end" {
			opening, found := lines.Line(parent)
			if !found || opening.Type() != "block-start" {
				return nil
			}
			owner, _ := opening.ParentLine()
			name, _ := v167Tag(opening)
			if owner == block.Number() && opening.NestingLevel() == block.NestingLevel()+1 && (name == "answer" || name == "hint") {
				return opening
			}
			return nil
		}
		if parent != block.Number() || line.NestingLevel() != block.NestingLevel()+1 || strings.Trim(line.Raw(), " \t") == "---" {
			return nil
		}
		if strings.Trim(line.Raw(), " \t") == "" {
			continue
		}
		// Незавершённый вложенный блок не даёт связи с предыдущим ответом.
		if line.Type() == "block-start" {
			return nil
		}
		return line
	}
	return nil
}

// v167Tag извлекает только объявление, распознанное моделью, без разбора Raw.
func v167Tag(line diagnostics.Line) (string, diagnostics.Element) {
	if line.Type() != "tag" && line.Type() != "block-start" && line.Type() != "invalid" {
		return "", nil
	}
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" {
			name, _ := element.Value()
			return strings.ToLower(name), element
		}
	}
	return "", nil
}

// v167LiteralParent исключает литеральные области editor и multifill.
func v167LiteralParent(lines diagnostics.Lines, line diagnostics.Line) bool {
	for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
		line, _ = lines.Line(parent)
		name, _ := v167Tag(line)
		if name == "editor" || name == "multifill" {
			return true
		}
	}
	return false
}
