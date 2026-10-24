package checkers

import "github.com/verdoga/dsl-validator/diagnostics"

// registrationV092 регистрирует рекомендацию о разделителе списка в DSL 1.2.
func registrationV092(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V092",
		Severity: diagnostics.SeverityRecommendation,
		Message:  "Однострочный список слов не содержит разделителя «;».",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV092,
	})
}

// checkV092 проверяет весь исходный остаток распознанного однострочного
// wordlist. Удаляются только крайние U+0020/U+0009, без декодирования.
// Диапазон содержимого задан в Unicode-колонках с 1, конец исключается.
func checkV092(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if line.Type() != "tag" {
			continue
		}
		tag := v092Tag(line)
		if tag == nil {
			continue
		}
		name, _ := tag.Value()
		if name != "wordlist" || v092LiteralContent(lines, line) {
			continue
		}
		raw := []rune(line.Raw())
		start, end := tag.End()-1, len(raw)
		for start < end && (raw[start] == ' ' || raw[start] == '\t') {
			start++
		}
		for end > start && (raw[end-1] == ' ' || raw[end-1] == '\t') {
			end--
		}
		if start == end || v092HasSeparator(raw[start:end]) {
			continue
		}
		if _, err := findings.Add(number, start+1, number, end+1,
			"Проверьте, не объединены ли несколько элементов без разделителя «;»."); err != nil {
			return err
		}
	}
	return nil
}

// v092Tag возвращает распознанное имя тега либо nil при его отсутствии.
// Объявление по Raw не восстанавливается.
func v092Tag(line diagnostics.Line) diagnostics.Element {
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" {
			return element
		}
	}
	return nil
}

// v092LiteralContent возвращает true для потомка блочного editor/multifill;
// false означает отсутствие такого контекста в готовой модели.
func v092LiteralContent(lines diagnostics.Lines, line diagnostics.Line) bool {
	for {
		parent, found := line.ParentLine()
		if !found {
			return false
		}
		line, _ = lines.Line(parent)
		if line.Type() != "block-start" {
			continue
		}
		if tag := v092Tag(line); tag != nil {
			name, _ := tag.Value()
			if name == "editor" || name == "multifill" {
				return true
			}
		}
	}
}

// v092HasSeparator возвращает true при наличии неэкранированной U+003B.
// false означает отсутствие разделителя; пары \\ и \; потребляются целиком,
// остальные символы читаются по одному, без изменения исходного содержимого.
func v092HasSeparator(content []rune) bool {
	for index := 0; index < len(content); index++ {
		if content[index] == '\\' && index+1 < len(content) &&
			(content[index+1] == '\\' || content[index+1] == ';') {
			index++
			continue
		}
		if content[index] == ';' {
			return true
		}
	}
	return false
}
