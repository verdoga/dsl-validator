package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV080 регистрирует отсутствие источника медиа в DSL 1.2.
func registrationV080(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V080",
		Severity: diagnostics.SeverityError,
		Message:  "У @media отсутствует путь или номер ресурса.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV080,
	})
}

// checkV080 самостоятельно выделяет имя и тип по исходным U+0020/U+0009.
// Непустой остаток после допустимого типа присутствует независимо от разбора,
// кавычек и значения source; декодирование и проверка файла не выполняются.
func checkV080(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if v080LiteralContent(lines, line) {
			continue
		}
		raw := []rune(line.Raw())
		start, end := v080Word(raw, 0)
		if strings.ToLower(string(raw[start:end])) != "@media" {
			continue
		}
		typeStart, typeEnd := v080Word(raw, end)
		switch strings.ToLower(string(raw[typeStart:typeEnd])) {
		case "audio", "video", "image":
		default:
			continue
		}
		if strings.Trim(string(raw[typeEnd:]), " \t") != "" {
			continue
		}
		if _, err := findings.Add(number, start+1, number, end+1, ""); err != nil {
			return err
		}
	}
	return nil
}

// v080Word возвращает границы следующего слова в кодовых точках с 0.
// Разделители — только U+0020/U+0009; конец исключается, start=end при отсутствии.
func v080Word(raw []rune, offset int) (start, end int) {
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

// v080LiteralContent возвращает true при наличии блочного editor/multifill
// среди предков. false означает, что объявление подлежит проверке.
// Границы блоков и родительство берутся из готовой модели.
func v080LiteralContent(lines diagnostics.Lines, line diagnostics.Line) bool {
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
			start, end := v080Word(raw, 0)
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
