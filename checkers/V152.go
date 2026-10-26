package checkers

import (
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV152 регистрирует пустые варианты ответа в multifill для DSL 1.2.
func registrationV152(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code: "V152", Severity: diagnostics.SeverityError, Scope: diagnostics.ScopeLine,
		Message:  "В пропуске указан пустой ответ.",
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV152,
	})
}

// checkV152 отмечает каждый пустой вариант от начала плейсхолдера до его закрытия.
// Координаты относятся к исходным рунам; нормализация не меняет документ.
func checkV152(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if !v152Prefilled(lines, line) {
			continue
		}
		for _, span := range v152EmptyAnswers([]rune(line.Raw())) {
			if _, err := findings.Add(line.Number(), span[0]+1, line.Number(), span[1]+1, ""); err != nil {
				return err
			}
		}
	}
	return nil
}

// v152Prefilled возвращает true для предзаполненного содержимого multifill.
// Ближайший фигурный родитель определяет контекст; границы блока дают false.
func v152Prefilled(lines diagnostics.Lines, line diagnostics.Line) bool {
	if line.Type() == "block-start" || line.Type() == "block-end" {
		return false
	}
	for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
		line, _ = lines.Line(parent)
		if line.Type() != "block-start" {
			continue
		}
		for i := 0; i < line.ElementCount(); i++ {
			element, _ := line.Element(i)
			if element.Type() == "tag" {
				name, _ := element.Value()
				return strings.EqualFold(name, "multifill")
			}
		}
		return false
	}
	return false
}

// v152EmptyAnswers возвращает исходные диапазоны [start,end) в рунах с нуля.
// Все соседние группы проверяются независимо; незакрытая группа завершает строку.
func v152EmptyAnswers(raw []rune) [][2]int {
	var spans [][2]int
	for i := 0; i < len(raw); {
		if raw[i] == '\\' && i+1 < len(raw) {
			if raw[i+1] == '_' {
				i += 2
				for i < len(raw) && raw[i] == '_' {
					i++
				}
				continue
			}
			if strings.ContainsRune("\\{}", raw[i+1]) {
				i += 2
				continue
			}
		}
		if raw[i] != '_' {
			i++
			continue
		}
		start := i
		for i < len(raw) && raw[i] == '_' {
			i++
		}
		if i-start != 5 || i == len(raw) || raw[i] != '{' {
			continue
		}
		for i < len(raw) && raw[i] == '{' {
			end, valid := v152AnswerEnd(raw, i)
			if end == 0 {
				return spans
			}
			if valid && v152Normalize(raw[i+1:end-1]) == "" {
				spans = append(spans, [2]int{start, end})
			}
			i = end
		}
	}
	return spans
}

// v152AnswerEnd возвращает исключающий конец группы в рунах с нуля, либо 0 без закрытия.
// valid=false исключает группу с внутренней неэкранированной { или без закрытия.
func v152AnswerEnd(raw []rune, opening int) (end int, valid bool) {
	valid = true
	for i := opening + 1; i < len(raw); i++ {
		if raw[i] == '\\' && i+1 < len(raw) && strings.ContainsRune("\\{}", raw[i+1]) {
			i++
			continue
		}
		if raw[i] == '{' {
			valid = false
		}
		if raw[i] == '}' {
			return i + 1, valid
		}
	}
	return 0, false
}

// v152Normalize снимает разрешённое экранирование, меняет регистр и нормализует пробелы.
// Unicode White_Space сжимается до U+0020 без краевых пробелов; пунктуация сохраняется.
func v152Normalize(raw []rune) string {
	var decoded strings.Builder
	for i := 0; i < len(raw); i++ {
		if raw[i] == '\\' && i+1 < len(raw) && strings.ContainsRune("\\{}", raw[i+1]) {
			i++
		}
		decoded.WriteRune(raw[i])
	}
	lower := strings.ToLower(decoded.String())
	return strings.Join(strings.FieldsFunc(lower, func(r rune) bool {
		return unicode.Is(unicode.White_Space, r)
	}), " ")
}
