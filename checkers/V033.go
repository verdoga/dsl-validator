package checkers

import (
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// v033Task хранит исходный диапазон ID и строки объявлений блока и ветви.
// Нули variants и branch означают отсутствие блока или известной ветви.
type v033Task struct {
	line, start, end int
	variants, branch int
}

// v033Parents описывает контекст существующей цепочки родителей строки.
type v033Parents struct {
	variants         int
	inBlock, literal bool
}

// registrationV033 регистрирует ошибку повторного ID задания для DSL 1.2.
func registrationV033(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V033",
		Severity: diagnostics.SeverityError,
		Message:  "ID задания повторяется в пределах одного шага.",
		Scope:    diagnostics.ScopeElement,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV033,
	})
}

// checkV033 сравнивает допустимые ID распознанных заданий в пределах шага.
// Разные ветви одного variants исключаются попарно; связанным становится
// первое подходящее предыдущее вхождение. Исходные значения не изменяются.
func checkV033(lines diagnostics.Lines, findings diagnostics.Findings) error {
	seen := make(map[string][]v033Task)
	branches := make(map[int]int)
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if line.Type() == "block-start" && v033Tag(line) == "variants" {
			branches[number] = 0
		}
		if line.Type() == "block-end" {
			parent, _ := line.ParentLine()
			delete(branches, parent)
		}
		if line.Type() == "heading" {
			if level := v033Element(line, "heading-level"); level != nil {
				if value, _ := level.Value(); value == "3" {
					clear(seen)
				}
			}
			continue
		}
		if line.Type() != "tag" {
			continue
		}
		tag := v033Tag(line)
		if tag != "task" && tag != "newpage" && tag != "variant" {
			continue
		}
		parents := v033ParentContext(lines, line)
		if parents.literal {
			continue
		}
		if tag == "variant" {
			// Ветвь задаётся последовательностью объявлений одного блока,
			// даже когда задания не являются потомками @variant.
			parent, _ := line.ParentLine()
			if parents.variants != 0 && parent == parents.variants {
				branches[parents.variants] = number
			}
			continue
		}
		if tag == "newpage" {
			// Допустимое объявление не имеет параметров и фигурных предков.
			if !parents.inBlock && line.ElementCount() == 1 {
				clear(seen)
			}
			continue
		}
		identifier := v033Element(line, "identifier")
		if identifier == nil {
			continue
		}
		value, present := identifier.Value()
		if !present || !v033ValidID(value) {
			continue
		}
		key := strings.ToLower(value)
		current := v033Task{
			line: number, start: identifier.Start(), end: identifier.End(),
			variants: parents.variants, branch: branches[parents.variants],
		}
		for _, previous := range seen[key] {
			if current.variants != 0 && current.variants == previous.variants &&
				current.branch != 0 && previous.branch != 0 && current.branch != previous.branch {
				continue
			}
			occurrence, err := findings.Add(current.line, current.start, current.line, current.end, "")
			if err != nil {
				return err
			}
			if err := findings.AddRelated(occurrence, previous.line, previous.start, previous.line, previous.end); err != nil {
				return err
			}
			break
		}
		seen[key] = append(seen[key], current)
	}
	return nil
}

// v033Element возвращает первый существующий элемент заданного типа или nil.
func v033Element(line diagnostics.Line, kind string) diagnostics.Element {
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == kind {
			return element
		}
	}
	return nil
}

// v033Tag возвращает каноническое значение элемента tag либо пустую строку.
func v033Tag(line diagnostics.Line) string {
	if tag := v033Element(line, "tag"); tag != nil {
		value, _ := tag.Value()
		return value
	}
	return ""
}

// v033ValidID возвращает true для непустой строки из Unicode-букв,
// десятичных цифр и . - _; остальные строки дают false без обрезки пробелов.
func v033ValidID(value string) bool {
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

// v033ParentContext читает существующую цепочку родителей без восстановления
// грамматики. Блоки различаются по строкам, не по отображаемым именам.
func v033ParentContext(lines diagnostics.Lines, line diagnostics.Line) v033Parents {
	var context v033Parents
	for number, found := line.ParentLine(); found; number, found = line.ParentLine() {
		line, _ = lines.Line(number)
		tag := v033Tag(line)
		if line.Type() == "block-start" {
			context.inBlock = true
			switch tag {
			case "editor", "multifill":
				context.literal = true
			case "variants":
				context.variants = number
			}
		}
	}
	return context
}
