package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// v191Tag хранит имя объявления и исходные Unicode-колонки с 1.
// Правая граница диапазона исключается.
type v191Tag struct {
	name       string
	start, end int
}

// registrationV191 регистрирует размещение variant вне variants только для DSL 1.2.
func registrationV191(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code: "V191", Severity: diagnostics.SeverityError, Scope: diagnostics.ScopeLine,
		Message:  "@variant находится вне контейнера @variants.",
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV191,
	})
}

// checkV191 учитывает открытия из модели и отдельные физические закрытия }:
// корневой заголовок меняет ParentLine потомков, но не закрывает фигурный блок,
// а последующая } может остаться в unparsed из-за потерянного контекста парсера.
// Результаты других диагностик не участвуют в выборе исключённых контекстов.
func checkV191(lines diagnostics.Lines, findings diagnostics.Findings) error {
	var blocks []diagnostics.Line
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if strings.Trim(line.Raw(), " \t") == "}" {
			if len(blocks) != 0 {
				blocks = blocks[:len(blocks)-1]
			}
			continue
		}
		parent := ""
		if len(blocks) != 0 {
			parent = v191Declaration(blocks[len(blocks)-1]).name
		}
		// Псевдообъявления в литеральном теле не открывают вложенных блоков.
		if parent == "editor" || parent == "multifill" {
			continue
		}
		tag := v191Declaration(line)
		if tag.name == "variant" && !v191Excluded(parent) {
			if _, err := findings.Add(line.Number(), tag.start, line.Number(), tag.end, ""); err != nil {
				return err
			}
		}
		if line.Type() == "block-start" {
			blocks = append(blocks, line)
		}
	}
	return nil
}

// v191Declaration распознаёт начальный tag без декодирования Raw.
// Без tag восстановление допускается только для invalid или строк с unparsed.
// Границы имени: конец строки, U+0020/U+0009, { и }. Пустое имя — не объявление.
func v191Declaration(line diagnostics.Line) v191Tag {
	raw := []rune(line.Raw())
	start := 0
	for start < len(raw) && (raw[start] == ' ' || raw[start] == '\t') {
		start++
	}
	if start == len(raw) || raw[start] != '@' {
		return v191Tag{}
	}
	erroneous, hasTag := line.Type() == "invalid", false
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" {
			hasTag = true
			if element.Start() == start+1 {
				name, _ := element.Value()
				return v191Tag{strings.ToLower(name), element.Start(), element.End()}
			}
		}
		erroneous = erroneous || element.Type() == "unparsed"
	}
	if hasTag || !erroneous {
		return v191Tag{}
	}
	end := start + 1
	for end < len(raw) && raw[end] != ' ' && raw[end] != '\t' && raw[end] != '{' && raw[end] != '}' {
		end++
	}
	return v191Tag{strings.ToLower(string(raw[start+1 : end])), start + 1, end + 1}
}

// v191Excluded объединяет допустимый variants и явные исключения таблицы V191.
func v191Excluded(parent string) bool {
	switch parent {
	case "variants", "example", "wordlist", "table", "script", "text", "key",
		"note", "alt", "hint", "instr", "fragment", "answer", "choice",
		"multichoice", "matching", "ordering":
		return true
	}
	return false
}
