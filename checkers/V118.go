package checkers

import (
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV118 регистрирует рекомендацию о соседних прописных строках text.
func registrationV118(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V118",
		Severity: diagnostics.SeverityRecommendation,
		Message:  "Несколько соседних строк написаны прописными буквами.",
		Scope:    diagnostics.ScopeBlock,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV118,
	})
}

// checkV118 собирает максимальные последовательности в физическом порядке.
// Любая неподходящая строка завершает последовательность, включая вложенные блоки.
func checkV118(lines diagnostics.Lines, findings diagnostics.Findings) error {
	first, last, start, end, parent := 0, 0, 0, 0, 0
	flush := func() error {
		if last > first {
			if _, err := findings.Add(first, start, last, end,
				"Проверьте, не разбит ли один заголовок на несколько строк. При подтверждении объедините его; для заголовка всего текста используйте TITLE"); err != nil {
				return err
			}
		}
		first, last = 0, 0
		return nil
	}
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		owner := v118TextParent(lines, line)
		raw := []rune(line.Raw())
		left, right := 0, len(raw)
		for left < right && (raw[left] == ' ' || raw[left] == '\t') {
			left++
		}
		for right > left && (raw[right-1] == ' ' || raw[right-1] == '\t') {
			right--
		}
		upper := false
		if owner != 0 {
			visible, valid := v118Visible(raw[left:right])
			upper = valid && v118Upper(visible)
		}
		if !upper || owner != parent {
			if err := flush(); err != nil {
				return err
			}
		}
		if upper {
			if first == 0 {
				first, start, parent = number, left+1, owner
			}
			last, end = number, right+1
		}
	}
	return flush()
}

// v118TextParent возвращает непосредственный блок text либо ноль.
// invalid и любой unparsed прерывают последовательность независимо от других ошибок.
func v118TextParent(lines diagnostics.Lines, line diagnostics.Line) int {
	if line.Type() != "content" {
		return 0
	}
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "unparsed" {
			return 0
		}
	}
	parent, found := line.ParentLine()
	if !found {
		return 0
	}
	block, _ := lines.Line(parent)
	if block.Type() != "block-start" {
		return 0
	}
	for index := 0; index < block.ElementCount(); index++ {
		element, _ := block.Element(index)
		if element.Type() == "tag" {
			name, _ := element.Value()
			if strings.EqualFold(name, "text") {
				return parent
			}
		}
	}
	return 0
}

// v118Visible снимает корректное парное выделение и текстовое экранирование.
// false означает ошибку HTML; отдельный br даёт пустой видимый текст.
// Расшифрованные символы не распознаются повторно как разметка.
func v118Visible(raw []rune) (string, bool) {
	if strings.EqualFold(string(raw), "<br>") {
		return "", true
	}
	var visible strings.Builder
	var stack []string
	for index := 0; index < len(raw); {
		if raw[index] == '\\' && index+1 < len(raw) {
			next := raw[index+1]
			if strings.ContainsRune("\\{}<>", next) ||
				(index == 0 && (next == '@' || next == '#')) ||
				(next == '_' && index+2 < len(raw) && raw[index+2] == '_') {
				visible.WriteRune(next)
				index += 2
				continue
			}
		}
		if raw[index] == '>' {
			return "", false
		}
		if raw[index] != '<' {
			visible.WriteRune(raw[index])
			index++
			continue
		}
		end := index + 1
		for end < len(raw) && raw[end] != '>' {
			end++
		}
		if end == len(raw) {
			return "", false
		}
		// Только точные допустимые маркеры могут пройти дальше. Любой иной
		// участок (включая атрибут с кавычками) делает всю строку неподходящей.
		marker := strings.ToLower(string(raw[index+1 : end]))
		switch marker {
		case "strong", "em", "u":
			stack = append(stack, marker)
		case "/strong", "/em", "/u":
			if len(stack) == 0 || stack[len(stack)-1] != marker[1:] {
				return "", false
			}
			stack = stack[:len(stack)-1]
		default:
			return "", false
		}
		index = end + 1
	}
	return visible.String(), len(stack) == 0
}

// v118Upper требует регистровую букву и отсутствие строчных букв Unicode.
// Titlecase удовлетворяет этому условию; числа и пунктуация его не заменяют.
func v118Upper(text string) bool {
	cased := false
	for _, r := range text {
		if unicode.IsLower(r) || unicode.Is(unicode.Other_Lowercase, r) {
			return false
		}
		if unicode.IsUpper(r) || unicode.IsTitle(r) || unicode.Is(unicode.Other_Uppercase, r) {
			cased = true
		}
	}
	return cased
}
