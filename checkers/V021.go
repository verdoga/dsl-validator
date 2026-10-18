package checkers

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV021 регистрирует рекомендацию о разрыве абзаца только для DSL 1.2.
func registrationV021(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V021",
		Severity: diagnostics.SeverityRecommendation,
		Message:  "Строка без завершающего знака продолжается строкой со строчной буквы.",
		Scope:    diagnostics.ScopeBlock,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV021,
	})
}

// checkV021 рассматривает соседние строки непосредственного содержимого.
// Границы контейнеров и любые конструкции, кроме blank и отдельного <br>
// в text, прерывают соседство. Диапазон включает полные Raw обеих строк
// и промежуток между ними; правая Unicode-колонка исключается.
func checkV021(lines diagnostics.Lines, findings diagnostics.Findings) error {
	previous, container := 0, 0
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		parent, _ := line.ParentLine()
		if parent != container {
			previous = 0
			container = parent
		}
		tag := v021ContainerTag(lines, parent)
		if tag == "" {
			previous = 0
			continue
		}
		if line.Type() == "blank" {
			continue
		}
		if line.Type() != "content" {
			previous = 0
			continue
		}
		if tag == "text" && strings.ToLower(strings.Trim(line.Raw(), " \t")) == "<br>" {
			continue
		}
		visible := v021Visible(line, tag)
		if previous != 0 && v021StartsLower(visible) {
			if _, err := findings.Add(previous, 1, number, utf8.RuneCountInString(line.Raw())+1,
				"Проверьте, не разделён ли один абзац на несколько строк"); err != nil {
				return err
			}
		}
		previous = 0
		if v021Unfinished(visible) {
			previous = number
		}
	}
	return nil
}

// v021ContainerTag возвращает каноническое имя целевого непосредственного
// родителя либо пустую строку. Предки выше родителя не включаются в поиск.
func v021ContainerTag(lines diagnostics.Lines, parent int) string {
	line, found := lines.Line(parent)
	if !found || line.Type() != "block-start" {
		return ""
	}
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() != "tag" {
			continue
		}
		tag, _ := element.Value()
		switch tag {
		case "multifill", "text", "key", "note", "alt", "script", "instr":
			return tag
		}
	}
	return ""
}

// v021Visible получает текст для анализа, не изменяя исходную строку.
// Сначала учитываются исходные границы и экранирование, затем прозрачные
// маркеры. Результат снятия экранирования повторно как разметка не читается.
// Ответы multifill исключаются по элементам штатной модели; обозначение
// пропуска сохраняется. HTML-сущности не декодируются.
func v021Visible(line diagnostics.Line, tag string) []rune {
	raw := []rune(line.Raw())
	placeholders := make(map[int]int)
	if tag == "multifill" {
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			if element.Type() == "placeholder" {
				placeholders[element.Start()-1] = element.End() - 1
			}
		}
	}
	start, end := 0, len(raw)
	for start < end && (raw[start] == ' ' || raw[start] == '\t') {
		start++
	}
	for end > start && (raw[end-1] == ' ' || raw[end-1] == '\t') {
		end--
	}
	visible := make([]rune, 0, end-start)
	for index := start; index < end; {
		if next, found := placeholders[index]; found {
			visible = append(visible, '_', '_', '_', '_', '_')
			index = next
			continue
		}
		if raw[index] == '\\' && index+1 < end {
			next := raw[index+1]
			if next == '_' && tag != "key" && index+2 < end && raw[index+2] == '_' {
				index++
				for index < end && raw[index] == '_' {
					visible = append(visible, '_')
					index++
				}
				continue
			}
			leading := index == start && (next == '@' || next == '#') && tag != "multifill"
			if index == start && tag == "multifill" && next == '@' {
				declaration := strings.ToLower(string(raw[index+1 : end]))
				leading = declaration == "@answer" || strings.HasPrefix(declaration, "@answer ") ||
					strings.HasPrefix(declaration, "@answer\t")
			}
			if next == '\\' || next == '{' || next == '}' || leading ||
				(tag == "text" && (next == '<' || next == '>')) {
				visible = append(visible, next)
				index += 2
				continue
			}
		}
		if tag == "text" && raw[index] == '<' {
			if length := v021HTMLLength(raw[index:end]); length != 0 {
				index += length
				continue
			}
		}
		visible = append(visible, raw[index])
		index++
	}
	return []rune(strings.Trim(string(visible), " \t"))
}

// v021HTMLLength возвращает длину поддерживаемого маркера либо ноль.
// Проверяется запись маркера, а не парность HTML. <br> здесь не прозрачен:
// отдельная корректная строка обрабатывается до получения видимого текста.
func v021HTMLLength(raw []rune) int {
	for _, marker := range [...]string{"<strong>", "</strong>", "<em>", "</em>", "<u>", "</u>"} {
		if len(raw) >= len(marker) && strings.ToLower(string(raw[:len(marker)])) == marker {
			return len(marker)
		}
	}
	return 0
}

// v021Unfinished возвращает true для строки с буквой, без завершающего
// знака или дефиса/тире после удаления завершающих Pe/Pf и ASCII-кавычек.
func v021Unfinished(text []rune) bool {
	hasLetter := false
	for _, char := range text {
		if unicode.IsLetter(char) {
			hasLetter = true
			break
		}
	}
	if !hasLetter {
		return false
	}
	end := len(text)
	for end > 0 && (unicode.Is(unicode.Pe, text[end-1]) || unicode.Is(unicode.Pf, text[end-1]) ||
		text[end-1] == '"' || text[end-1] == '\'') {
		end--
	}
	if end == 0 {
		return false
	}
	last := text[end-1]
	return last != '.' && last != '?' && last != '!' && last != '…' && !unicode.Is(unicode.Pd, last)
}

// v021StartsLower возвращает true, когда после открывающих Ps/Pi и
// ASCII-кавычек находится Ll. Пробелы внутри текста не удаляются.
func v021StartsLower(text []rune) bool {
	for _, char := range text {
		if unicode.Is(unicode.Ps, char) || unicode.Is(unicode.Pi, char) || char == '"' || char == '\'' {
			continue
		}
		return unicode.Is(unicode.Ll, char)
	}
	return false
}
