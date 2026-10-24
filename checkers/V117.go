package checkers

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV117 регистрирует редакторскую эвристику только для DSL 1.2.
func registrationV117(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code: "V117", Severity: diagnostics.SeverityRecommendation,
		Message:  "В тексте есть разделённые обычным текстом строки из прописных букв.",
		Scope:    diagnostics.ScopeBlock,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV117,
	})
}

// checkV117 проверяет каждый text независимо, сохраняя порядок исходных блоков.
func checkV117(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if line.Type() != "block-start" {
			continue
		}
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			name, present := element.Value()
			if element.Type() == "tag" && present && strings.EqualFold(name, "text") {
				if err := v117Block(lines, findings, number); err != nil {
					return err
				}
				break
			}
		}
	}
	return nil
}

// v117Block выбирает первую подходящую пару групп. Обычная строка между
// кандидатами гарантирует, что они принадлежат разным физическим группам.
// Пустота и ошибочная HTML-разметка сами не доказывают наличие обычного текста.
func v117Block(lines diagnostics.Lines, findings diagnostics.Findings, block int) error {
	first, end := 0, block
	ordinary := false
	var pair [2]int
	for number := block + 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if !v117Descendant(lines, line, block) {
			break
		}
		end = number
		parent, _ := line.ParentLine()
		if parent != block {
			continue
		}
		if line.Type() == "block-end" {
			break
		}
		if pair[0] != 0 {
			continue // Границу блока продолжаем искать после найденной пары.
		}
		if line.Type() == "blank" {
			continue
		}
		if line.Type() != "content" {
			first, ordinary = 0, false
			continue // Вложенное объявление прерывает сравнение внешнего текста.
		}
		visible, valid := v117Visible(line.Raw())
		if !valid || strings.Trim(string(visible), " \t") == "" {
			continue
		}
		cased, lower := false, false
		for _, char := range visible {
			isLower := unicode.IsLower(char) || unicode.Is(unicode.Other_Lowercase, char)
			lower = lower || isLower
			cased = cased || isLower || unicode.IsUpper(char) || unicode.IsTitle(char) || unicode.Is(unicode.Other_Uppercase, char)
		}
		if cased && !lower {
			if first == 0 {
				first = number
			} else if ordinary {
				pair = [2]int{first, number}
			}
		} else if first != 0 {
			ordinary = true
		}
	}
	if pair[0] == 0 {
		return nil
	}
	last, _ := lines.Line(end)
	occurrence, err := findings.Add(block, 1, end, utf8.RuneCountInString(last.Raw())+1,
		"Проверьте, не объединены ли здесь несколько самостоятельных текстов. При подтверждении разделите их на отдельные @text с собственными названиями")
	if err != nil {
		return err
	}
	for _, number := range pair {
		line, _ := lines.Line(number)
		if err := findings.AddRelated(occurrence, number, 1, number, utf8.RuneCountInString(line.Raw())+1); err != nil {
			return err
		}
	}
	return nil
}

// v117Descendant проверяет готовые родительские связи, не восстанавливая парсер.
func v117Descendant(lines diagnostics.Lines, line diagnostics.Line, block int) bool {
	for {
		parent, found := line.ParentLine()
		if !found || parent < block {
			return false
		}
		if parent == block {
			return true
		}
		line, _ = lines.Line(parent)
	}
}

// v117Visible снимает только корректные парные strong/em/u в одной строке.
// valid=false означает ошибочную HTML-разметку; отдельный br даёт пустой текст.
// Экранированные знаки выводятся буквально и повторно не распознаются.
func v117Visible(source string) (visible []rune, valid bool) {
	source = strings.Trim(source, " \t")
	if strings.EqualFold(source, "<br>") {
		return nil, true
	}
	raw := []rune(source)
	var stack []string
	for index := 0; index < len(raw); index++ {
		char := raw[index]
		if char == '\\' && index+1 < len(raw) {
			next := raw[index+1]
			if strings.ContainsRune("\\{}<>", next) || index == 0 && (next == '@' || next == '#') ||
				next == '_' && index+2 < len(raw) && raw[index+2] == '_' {
				visible = append(visible, next)
				index++
				continue
			}
		}
		if char == '>' {
			return nil, false
		}
		if char != '<' {
			visible = append(visible, char)
			continue
		}
		end := index + 1
		for end < len(raw) && raw[end] != '>' {
			end++
		}
		if end == len(raw) {
			return nil, false
		}
		name := strings.ToLower(string(raw[index+1 : end]))
		closing := strings.HasPrefix(name, "/")
		if closing {
			name = name[1:]
		}
		if name != "strong" && name != "em" && name != "u" {
			return nil, false // Атрибуты, самозакрытие и другие имена недопустимы.
		}
		if closing {
			if len(stack) == 0 || stack[len(stack)-1] != name {
				return nil, false
			}
			stack = stack[:len(stack)-1]
		} else {
			stack = append(stack, name)
		}
		index = end
	}
	return visible, len(stack) == 0
}
