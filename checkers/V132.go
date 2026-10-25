package checkers

import (
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// v132Definition сохраняет ключ сравнения и исходный диапазон ID определения.
// Колонки начинаются с 1, правая граница исключается.
type v132Definition struct {
	id               string
	line, start, end int
}

// registrationV132 регистрирует обязательное использование фрагментов для DSL 1.2.
func registrationV132(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V132",
		Severity: diagnostics.SeverityError,
		Message:  "На @fragment нет ссылок @include.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV132,
	})
}

// checkV132 сопоставляет определения и явные ссылки во всём документе.
// Включения не разворачиваются; ветви, порядок и достижимость не ограничивают
// поиск. Повторяющиеся ID исключаются целиком, выдача идёт в порядке исходника.
func checkV132(lines diagnostics.Lines, findings diagnostics.Findings) error {
	var definitions []v132Definition
	counts, references := make(map[string]int), make(map[string]int)
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		tag := v132Tag(line)
		if tag == nil {
			continue
		}
		name, _ := tag.Value()
		if name != "fragment" && name != "include" || v132Literal(lines, line) {
			continue
		}
		identifier := v132FirstID(line, tag)
		if identifier == nil {
			continue
		}
		value, present := identifier.Value()
		if !present || !v132ValidID(value) {
			continue
		}
		id := v132IDKey(value)
		if name == "include" {
			references[id]++
			continue
		}
		counts[id]++
		definitions = append(definitions, v132Definition{id, number, identifier.Start(), identifier.End()})
	}
	for _, definition := range definitions {
		if counts[definition.id] != 1 || references[definition.id] != 0 {
			continue
		}
		if _, err := findings.Add(definition.line, definition.start, definition.line, definition.end,
			"Добавьте необходимое включение либо удалите неиспользуемое определение после редакторской проверки"); err != nil {
			return err
		}
	}
	return nil
}

// v132IDKey выбирает наименьшую руну каждого цикла Unicode simple folding.
// Ключи совпадают ровно при strings.EqualFold; индексы не требуют попарного
// сравнения ID. Обрезка, NFC, нормализация ширины и полное folding не применяются.
func v132IDKey(value string) string {
	return strings.Map(func(char rune) rune {
		key := char
		for folded := unicode.SimpleFold(char); folded != char; folded = unicode.SimpleFold(folded) {
			if folded < key {
				key = folded
			}
		}
		return key
	}, value)
}

// v132Tag возвращает сохранённый начальный tag либо nil; из unparsed ничего
// не восстанавливает. Начальные U+0020/U+0009 не входят в имя объявления.
func v132Tag(line diagnostics.Line) diagnostics.Element {
	raw := []rune(line.Raw())
	start := 0
	for start < len(raw) && (raw[start] == ' ' || raw[start] == '\t') {
		start++
	}
	if start == len(raw) || raw[start] != '@' {
		return nil
	}
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "tag" && element.Start() == start+1 {
			return element
		}
	}
	return nil
}

// v132FirstID возвращает сохранённый identifier первого аргумента либо nil.
// Raw проверяет только положение и границы: нельзя принять поздний ID или
// допустимый префикс ошибочного аргумента. Значение не извлекается из исходника.
// Ошибочный хвост после разделителя не отменяет первый аргумент.
func v132FirstID(line diagnostics.Line, tag diagnostics.Element) diagnostics.Element {
	raw := []rune(line.Raw())
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() != "identifier" || element.Start() < tag.End() {
			continue
		}
		for _, char := range raw[tag.End()-1 : element.Start()-1] {
			if char != ' ' && char != '\t' {
				return nil
			}
		}
		end := element.End() - 1
		if end < len(raw) && raw[end] != ' ' && raw[end] != '\t' && raw[end] != '{' && raw[end] != '}' {
			return nil
		}
		return element
	}
	return nil
}

// v132ValidID возвращает true для непустого ID из Unicode-букв, десятичных
// цифр и . - _. Остальные значения дают false без обрезки и декодирования.
func v132ValidID(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if !unicode.IsLetter(char) && !unicode.IsDigit(char) && char != '.' && char != '-' && char != '_' {
			return false
		}
	}
	return true
}

// v132Literal возвращает true для потомков блочных editor/multifill,
// false — для остальных контекстов. Читается вся готовая цепочка родителей.
func v132Literal(lines diagnostics.Lines, line diagnostics.Line) bool {
	for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
		line, _ = lines.Line(parent)
		if line.Type() != "block-start" {
			continue
		}
		if tag := v132Tag(line); tag != nil {
			name, _ := tag.Value()
			if name == "editor" || name == "multifill" {
				return true
			}
		}
	}
	return false
}
