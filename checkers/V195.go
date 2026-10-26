package checkers

import (
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// v195Block хранит физическое открытие блока и диапазон имени тега.
// Колонки Unicode начинаются с 1; конец диапазона исключается.
type v195Block struct {
	name             string
	line, start, end int
}

// registrationV195 регистрирует заголовки внутри variants только для DSL 1.2.
func registrationV195(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code: "V195", Severity: diagnostics.SeverityError, Scope: diagnostics.ScopeLine,
		Message:  "Внутри @variants записан заголовок с #.",
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV195,
	})
}

// checkV195 ведёт физические интервалы независимо от родителей заголовков.
// Заголовок не завершает блок; незакрытый интервал продолжается до конца файла.
func checkV195(lines diagnostics.Lines, findings diagnostics.Findings) error {
	var stack []v195Block
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		literal := len(stack) > 0 && (stack[len(stack)-1].name == "editor" || stack[len(stack)-1].name == "multifill")
		// В литеральном содержимом закрытием служит только отдельная }.
		closing := strings.Trim(line.Raw(), " \t") == "}"
		if !literal && line.Type() == "block-end" {
			closing = true
		}
		if closing {
			if parent, found := line.ParentLine(); found && !literal && line.Type() == "block-end" {
				for index := len(stack) - 1; index >= 0; index-- {
					if stack[index].line == parent {
						stack = stack[:index]
						closing = false
						break
					}
				}
			}
			if closing && len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			continue
		}
		if literal {
			continue
		}
		start, end := v195Heading(line.Raw())
		if start != 0 && len(stack) > 0 && stack[len(stack)-1].name != "table" {
			for index := len(stack) - 1; index >= 0; index-- {
				block := stack[index]
				if block.name != "variants" {
					continue
				}
				occurrence, err := findings.Add(line.Number(), start, line.Number(), end, "")
				if err != nil {
					return err
				}
				if err := findings.AddRelated(occurrence, block.line, block.start, block.line, block.end); err != nil {
					return err
				}
				break
			}
		}
		if block, found := v195Opening(line); found {
			stack = append(stack, block)
		}
	}
	return nil
}

// v195Heading возвращает исходные колонки [start,end) либо нули для не-заголовка.
// После 1–3 решёток обязателен White_Space; обрезаются только U+0020/U+0009.
func v195Heading(raw string) (start, end int) {
	runes := []rune(raw)
	first := 0
	for first < len(runes) && (runes[first] == ' ' || runes[first] == '\t') {
		first++
	}
	after := first
	for after < len(runes) && runes[after] == '#' {
		after++
	}
	if after-first < 1 || after-first > 3 || after == len(runes) || !unicode.IsSpace(runes[after]) {
		return 0, 0
	}
	end = len(runes)
	for end > after && (runes[end-1] == ' ' || runes[end-1] == '\t') {
		end--
	}
	return first + 1, end + 1
}

// v195Opening читает структурное открытие, а при неполном разборе восстанавливает
// его по начальному имени блочного тега и неэкранированной {. found=false означает
// отсутствие открытия. Литеральные контексты исключаются вызывающим кодом.
func v195Opening(line diagnostics.Line) (block v195Block, found bool) {
	raw := []rune(line.Raw())
	start := 0
	for start < len(raw) && (raw[start] == ' ' || raw[start] == '\t') {
		start++
	}
	if start == len(raw) || raw[start] != '@' {
		return block, false
	}
	end := start + 1
	for end < len(raw) && !unicode.IsSpace(raw[end]) && raw[end] != '{' && raw[end] != '}' {
		end++
	}
	block = v195Block{strings.ToLower(string(raw[start+1 : end])), line.Number(), start + 1, end + 1}
	hasOpen := false
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" && element.Start() == start+1 {
			name, _ := element.Value()
			block.name, block.end = strings.ToLower(name), element.End()
			end = element.End() - 1
		}
		if element.Type() == "block-open" {
			hasOpen = true
		}
	}
	if hasOpen {
		return block, true
	}
	if !v195BlockName(block.name) {
		return block, false
	}
	for index := end; index < len(raw); index++ {
		if raw[index] == '\\' && index+1 < len(raw) &&
			(raw[index+1] == '\\' || raw[index+1] == '{' || raw[index+1] == '}') {
			index++
			continue
		}
		if raw[index] == '{' {
			return block, true
		}
	}
	return block, false
}

// v195BlockName ограничивает восстановление открытия блочными тегами DSL 1.2.
func v195BlockName(name string) bool {
	switch name {
	case "editor", "example", "wordlist", "table", "script", "text", "key",
		"instr", "note", "alt", "hint", "fragment", "answer", "multifill",
		"choice", "multichoice", "matching", "ordering", "variants":
		return true
	}
	return false
}
