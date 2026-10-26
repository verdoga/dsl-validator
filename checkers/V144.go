package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV144 регистрирует пунктуационную эвристику question для DSL 1.2.
func registrationV144(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code: "V144", Severity: diagnostics.SeverityWarning, Scope: diagnostics.ScopeLine,
		Message:  "В @question несколько фрагментов с вопросительными знаками.",
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV144,
	})
}

// checkV144 выдаёт одно предупреждение на весь текст с двумя и более завершениями.
// Связанные фрагменты включают завершающую группу и исходные начальные пробелы.
func checkV144(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		start, end := v144Text(line)
		if start == end || v144Literal(lines, line) {
			continue
		}
		fragments := v144Fragments([]rune(line.Raw()), start, end)
		if len(fragments) < 2 {
			continue
		}
		occurrence, err := findings.Add(number, start+1, number, end+1,
			"Проверьте, не объединены ли самостоятельные вопросы. Если каждому требуется отдельный ответ, оформите их отдельными @question")
		if err != nil {
			return err
		}
		for _, fragment := range fragments {
			if err := findings.AddRelated(occurrence, number, fragment[0]+1, number, fragment[1]+1); err != nil {
				return err
			}
		}
	}
	return nil
}

// v144Tag возвращает единственный распознанный tag либо nil при неопределённости.
func v144Tag(line diagnostics.Line) diagnostics.Element {
	var tag diagnostics.Element
	for i := 0; i < line.ElementCount(); i++ {
		element, _ := line.Element(i)
		if element.Type() == "tag" {
			if tag != nil {
				return nil
			}
			tag = element
		}
	}
	return tag
}

// v144Text возвращает диапазон текста [start,end) в исходных рунах с нуля.
// Нулевой диапазон исключает строку. Content и placeholder задают общий диапазон;
// пробелы между ними сохраняются, Value не декодируется и не подменяет Raw.
func v144Text(line diagnostics.Line) (start, end int) {
	if line.Type() != "tag" {
		return 0, 0
	}
	tag := v144Tag(line)
	if tag == nil {
		return 0, 0
	}
	name, _ := tag.Value()
	indent := len(line.Raw()) - len(strings.TrimLeft(line.Raw(), " \t"))
	if !strings.EqualFold(name, "question") || tag.Start() != indent+1 || !strings.HasPrefix(line.Raw()[indent:], "@") {
		return 0, 0
	}
	for i := 0; i < line.ElementCount(); i++ {
		element, _ := line.Element(i)
		if element.Type() == "block-open" {
			return 0, 0
		}
		if (element.Type() == "content" || element.Type() == "placeholder") && element.Start() >= tag.End() {
			if start == 0 {
				start = element.Start() - 1
			}
			end = element.End() - 1
		}
	}
	return start, end
}

// v144Literal возвращает true внутри editor/multifill, иначе false.
// Проверяется вся готовая цепочка родителей без восстановления блоков.
func v144Literal(lines diagnostics.Lines, line diagnostics.Line) bool {
	for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
		line, _ = lines.Line(parent)
		if line.Type() == "block-start" {
			if tag := v144Tag(line); tag != nil {
				name, _ := tag.Value()
				if strings.EqualFold(name, "editor") || strings.EqualFold(name, "multifill") {
					return true
				}
			}
		}
	}
	return false
}

// v144Fragments выделяет завершения по непрерывным группам ASCII .!?.
// Диапазоны исходных рун с нуля имеют исключающий конец. По уточнению пользователя
// буквы и цифры перед группой не требуются: ? ? — два завершения, ?!? — одно.
func v144Fragments(raw []rune, start, end int) [][2]int {
	var fragments [][2]int
	first := start
	for i := start; i < end; {
		if !strings.ContainsRune(".!?", raw[i]) {
			i++
			continue
		}
		question := false
		for i < end && strings.ContainsRune(".!?", raw[i]) {
			question = question || raw[i] == '?'
			i++
		}
		if question {
			fragments = append(fragments, [2]int{first, i})
		}
		first = i
	}
	return fragments
}
