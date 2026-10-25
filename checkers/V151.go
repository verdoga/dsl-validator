package checkers

import (
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV151 регистрирует внутренние открывающие скобки ответов DSL 1.2.
func registrationV151(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V151",
		Severity: diagnostics.SeverityError,
		Message:  "Внутри ответа пропуска есть неэкранированная открывающая скобка.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV151,
	})
}

// checkV151 проверяет Raw предзаполненных строк без декодирования и склейки.
// Состояние ответа ограничено физической строкой; ошибки его закрытия сюда не входят.
func checkV151(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if !v151InMultifill(lines, line) {
			continue
		}
		raw := []rune(line.Raw())
		for i := 0; i < len(raw); {
			if raw[i] == '\\' && i+1 < len(raw) {
				if raw[i+1] == '_' {
					// По п. 4.3.2 экранируется вся непрерывная последовательность.
					i++
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
			var err error
			i, err = v151Answers(raw, i+1, number, findings)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// v151InMultifill возвращает true для содержимого ближайшего фигурного multifill.
// Границы и родители берутся из модели; Raw нужен лишь для имени объявления.
// Инструкция самого блока, закрытие и псевдообъявления внутри editor исключены.
func v151InMultifill(lines diagnostics.Lines, line diagnostics.Line) bool {
	if line.Type() == "block-end" {
		return false
	}
	for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
		line, _ = lines.Line(parent)
		if line.Type() != "block-start" {
			continue
		}
		raw := strings.TrimLeft(line.Raw(), " \t")
		end := strings.IndexFunc(raw, func(char rune) bool {
			return unicode.Is(unicode.White_Space, char) || char == '{' || char == '}'
		})
		if end >= 0 {
			raw = raw[:end]
		}
		return strings.EqualFold(raw, "@multifill")
	}
	return false
}

// v151Answers просматривает ответ от start и все непосредственно соседние группы.
// Возвращает индекс следующего исходного символа либо len(raw). Первая
// неэкранированная } закрывает группу независимо от найденных внутренних {.
// Диапазон каждого сообщения — одна кодовая точка, конец исключён.
func v151Answers(raw []rune, start, number int, findings diagnostics.Findings) (int, error) {
	for i := start; i < len(raw); {
		if raw[i] == '\\' && i+1 < len(raw) && strings.ContainsRune("\\{}", raw[i+1]) {
			i += 2
			continue
		}
		switch raw[i] {
		case '{':
			if _, err := findings.Add(number, i+1, number, i+2, ""); err != nil {
				return i, err
			}
		case '}':
			i++
			if i < len(raw) && raw[i] == '{' {
				// _____{A}{B}: эта скобка открывает следующий допустимый вариант.
				i++
				continue
			}
			return i, nil
		}
		i++
	}
	return len(raw), nil
}
