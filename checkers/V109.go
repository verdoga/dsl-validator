package checkers

import (
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV109 регистрирует ошибочную блочную форму однострочных тегов.
func registrationV109(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V109",
		Severity: diagnostics.SeverityError,
		Message:  "У однострочного тега использована блочная форма.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV109,
	})
}

// checkV109 отмечает одно открытие на объявление, независимо от парности.
// Основной диапазон охватывает {, связанный — исходное имя тега.
func checkV109(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if v109Literal(lines, line) {
			continue
		}
		raw := []rune(line.Raw())
		name, start, end := v109Tag(line, raw)
		switch name {
		case "dsl-version", "document-id", "section", "order", "resource-dir",
			"header", "task", "endtask", "step", "speaking", "newpage",
			"media", "include", "question", "variant":
		default:
			continue
		}
		opening := v109Opening(line, raw, name, end)
		if opening < 0 {
			continue
		}
		occurrence, err := findings.Add(number, opening+1, number, opening+2,
			"Тег @"+name+" поддерживает только однострочную запись")
		if err != nil {
			return err
		}
		if err := findings.AddRelated(occurrence, number, start+1, number, end+1); err != nil {
			return err
		}
	}
	return nil
}

// v109Tag возвращает каноническое начальное имя и его диапазон в кодовых
// точках с 0, конец исключён. Пустое имя означает отсутствие объявления.
func v109Tag(line diagnostics.Line, raw []rune) (name string, start, end int) {
	for start < len(raw) && (raw[start] == ' ' || raw[start] == '\t') {
		start++
	}
	if start == len(raw) || raw[start] != '@' {
		return "", start, start
	}
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" && element.Start() == start+1 {
			name, _ = element.Value()
			return strings.ToLower(name), start, element.End() - 1
		}
	}
	end = start + 1
	for end < len(raw) && !unicode.IsSpace(raw[end]) && raw[end] != '{' && raw[end] != '}' {
		end++
	}
	return strings.ToLower(string(raw[start+1 : end])), start, end
}

// v109Literal возвращает true для потомков блочных editor/multifill,
// false — для остальных контекстов. Используется вся цепочка родителей.
func v109Literal(lines diagnostics.Lines, line diagnostics.Line) bool {
	for {
		parent, found := line.ParentLine()
		if !found {
			return false
		}
		line, _ = lines.Line(parent)
		if line.Type() == "block-start" {
			name, _, _ := v109Tag(line, []rune(line.Raw()))
			if name == "editor" || name == "multifill" {
				return true
			}
		}
	}
}

// v109Opening возвращает индекс открывающей скобки в исходных кодовых
// точках с 0 либо -1. Распознанное открытие имеет приоритет перед поиском
// по Raw; диапазоны source/resource-path защищают значения путей.
func v109Opening(line diagnostics.Line, raw []rune, name string, tagEnd int) int {
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "block-open" && !v109InPath(line, name, element.Start()-1) {
			return element.Start() - 1
		}
	}
	opening := v109TrailingOpening(raw, tagEnd)
	if opening < 0 || v109InPath(line, name, opening) {
		return -1
	}
	switch name {
	case "media":
		if end := v109SourceEnd(line, raw, tagEnd); end < 0 || end > opening {
			return -1
		}
	case "resource-dir":
		if !v109AfterQuotedPath(raw, tagEnd, opening) {
			return -1
		}
	}
	return opening
}

// v109InPath возвращает true, если индекс принадлежит распознанному пути
// соответствующего тега; false означает отсутствие такого диапазона.
func v109InPath(line diagnostics.Line, name string, index int) bool {
	for i := 0; i < line.ElementCount(); i++ {
		element, _ := line.Element(i)
		if (name == "media" && element.Type() == "source" ||
			name == "resource-dir" && element.Type() == "resource-path") &&
			index >= element.Start()-1 && index < element.End()-1 {
			return true
		}
	}
	return false
}

// v109TrailingOpening ищет завершающую активную { после U+0020.
// Возвращает исходный индекс с 0 либо -1. Кавычки обычного текста не меняют
// обработку; пары \\, \{ и \} потребляются целиком.
func v109TrailingOpening(raw []rune, tagEnd int) int {
	end := len(raw)
	for end > tagEnd && (raw[end-1] == ' ' || raw[end-1] == '\t') {
		end--
	}
	if end <= tagEnd+1 || raw[end-1] != '{' || raw[end-2] != ' ' {
		return -1
	}
	for index := tagEnd; index < end; index++ {
		if raw[index] == '\\' && index+1 < end &&
			(raw[index+1] == '\\' || raw[index+1] == '{' || raw[index+1] == '}') {
			index++
		} else if index == end-1 {
			return index
		}
	}
	return -1
}

// v109SourceEnd возвращает исключённый конец полного SOURCE media в
// кодовых точках с 0 либо -1 при отсутствии/незакрытых кавычках.
// Без элемента source имя, TYPE и SOURCE разделяются U+0020 по DSL 1.29.
// Значение TYPE здесь не проверяется: это другая диагностика.
func v109SourceEnd(line diagnostics.Line, raw []rune, tagEnd int) int {
	start := -1
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "source" {
			start = element.Start() - 1
			break
		}
	}
	if start < 0 {
		if tagEnd == len(raw) || raw[tagEnd] != ' ' {
			return -1
		}
		start = tagEnd
		for start < len(raw) && raw[start] == ' ' {
			start++
		}
		for start < len(raw) && raw[start] != ' ' {
			start++
		}
		for start < len(raw) && raw[start] == ' ' {
			start++
		}
	}
	if start >= len(raw) {
		return -1
	}
	if raw[start] != '"' {
		for start < len(raw) && raw[start] != ' ' {
			start++
		}
		return start
	}
	for index := start + 1; index < len(raw); index++ {
		if raw[index] == '\\' && index+1 < len(raw) {
			switch raw[index+1] {
			case '\\', '"', '{', '}':
				index++
				continue
			}
		}
		if raw[index] == '"' {
			return index + 1
		}
	}
	return -1
}

// v109AfterQuotedPath возвращает true только для отдельной { после
// закрытой кавычки последнего пути и одного или нескольких U+0020.
// false означает другой хвост. Запятые внутри кавычек и обратные косые
// черты буквальны; незаключённый путь читается до запятой, включая скобки.
func v109AfterQuotedPath(raw []rune, tagEnd, opening int) bool {
	index := tagEnd
	for index < opening {
		for index < opening && (raw[index] == ' ' || raw[index] == '\t') {
			index++
		}
		if index == opening {
			return false
		}
		if raw[index] == '"' {
			index++
			for index < opening && raw[index] != '"' {
				index++
			}
			if index == opening {
				return false
			}
			index++
			end := index
			for index < opening && raw[index] == ' ' {
				index++
			}
			if index == opening {
				return index > end
			}
			for index < opening && (raw[index] == ' ' || raw[index] == '\t') {
				index++
			}
			if index == opening || raw[index] != ',' {
				return false
			}
		} else {
			for index < opening && raw[index] != ',' {
				index++
			}
			if index == opening {
				return false
			}
		}
		index++
	}
	return false
}
