package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV085 регистрирует проверку обратных косых черт в путях медиа DSL 1.2.
func registrationV085(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V085",
		Severity: diagnostics.SeverityError,
		Message:  "Литеральная обратная косая черта в пути медиа не удвоена.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV085,
	})
}

// checkV085 читает заключённый в кавычки путь прямо из исходника, включая
// неразобранный текст. Диапазоны отдельных черт заданы в кодовых точках с 1,
// с исключённой правой границей; декодирование пути не выполняется.
func checkV085(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if v085LiteralContent(lines, line) {
			continue
		}
		raw := []rune(line.Raw())
		start, end := v085Word(raw, 0)
		if strings.ToLower(string(raw[start:end])) != "@media" {
			continue
		}
		typeStart, typeEnd := v085Word(raw, end)
		switch strings.ToLower(string(raw[typeStart:typeEnd])) {
		case "audio", "video", "image":
		default:
			continue
		}
		sourceStart, _ := v085Word(raw, typeEnd)
		if sourceStart == len(raw) || raw[sourceStart] != '"' {
			continue
		}
		for pos := sourceStart + 1; pos < len(raw); pos++ {
			if raw[pos] == '"' {
				break
			}
			if raw[pos] != '\\' {
				continue
			}
			if pos+1 < len(raw) {
				switch raw[pos+1] {
				case '\\', '"', '{', '}':
					pos++
					continue
				}
			}
			if _, err := findings.Add(number, pos+1, number, pos+2,
				"Для буквальной обратной косой черты используйте \\\\."); err != nil {
				return err
			}
		}
	}
	return nil
}

// v085Word возвращает границы следующего слова в кодовых точках с 0.
// Разделители — только U+0020/U+0009; конец исключается, start=end при отсутствии.
func v085Word(raw []rune, offset int) (start, end int) {
	start = offset
	for start < len(raw) && (raw[start] == ' ' || raw[start] == '\t') {
		start++
	}
	end = start
	for end < len(raw) && raw[end] != ' ' && raw[end] != '\t' {
		end++
	}
	return start, end
}

// v085LiteralContent возвращает true при наличии блочного editor/multifill
// среди предков. false означает, что объявление подлежит проверке.
// Границы блоков и родительство берутся из готовой модели.
func v085LiteralContent(lines diagnostics.Lines, line diagnostics.Line) bool {
	for {
		parent, found := line.ParentLine()
		if !found {
			return false
		}
		line, _ = lines.Line(parent)
		if line.Type() != "block-start" {
			continue
		}
		name := ""
		hasTag := false
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			if element.Type() == "tag" {
				name, _ = element.Value()
				hasTag = true
				break
			}
		}
		if !hasTag {
			raw := []rune(line.Raw())
			start, end := v085Word(raw, 0)
			// Частичный разбор мог сохранить блок с ошибочным @editor{.
			opening, _, _ := strings.Cut(strings.ToLower(string(raw[start:end])), "{")
			if opening == "@editor" || opening == "@multifill" {
				return true
			}
		}
		if name == "editor" || name == "multifill" {
			return true
		}
	}
}
