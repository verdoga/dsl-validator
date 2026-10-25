package checkers

import (
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV147 регистрирует отсутствие владельца ответа только для DSL 1.2.
func registrationV147(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code: "V147", Severity: diagnostics.SeverityError,
		Message:  "Для @answer нет допустимого вопроса или элемента, к которому он относится.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV147,
	})
}

// checkV147 определяет владельца по структуре, не используя значения ответов.
// questionAnswers хранит только непрерывные цепочки ответов соседнего вопроса:
// это позволяет исключить V146 без дальнего поиска вопроса для каждого ответа.
func checkV147(lines diagnostics.Lines, findings diagnostics.Findings) error {
	questionAnswers := make(map[int]bool)
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		name, start, end := v147Name(line)
		if name != "answer" || v147Literal(lines, line) {
			continue
		}
		block := v147BlockParent(lines, line)
		parentName, _, _ := v147Name(block)
		switch parentName {
		case "example", "wordlist", "table", "script", "text", "key", "instr", "note", "alt", "hint", "fragment", "answer":
			// Запрещённое вложение относится к V124 или V140.
			continue
		case "choice", "multichoice":
			continue
		}
		if parentName == "matching" && v147AfterMatchingSeparator(lines, block, line) {
			// Нижняя часть matching проверяется V166, без поиска владельца здесь.
			continue
		}
		previous := v147Previous(lines, line)
		previousName, _, _ := v147Name(previous)
		if previousName == "multifill" {
			// V142: непосредственная попытка связать ответ с multifill.
			continue
		}
		if parentName == "matching" || parentName == "ordering" {
			if v147Item(previous) && v147BeforeSeparator(lines, block, line.Number(), parentName == "ordering") {
				continue
			}
		} else if v147QuestionContext(lines, line) {
			if previousName == "question" && !v147Block(previous) ||
				previousName == "answer" && questionAnswers[previous.Number()] {
				questionAnswers[number] = true
				continue
			}
			// V145: ровно одна промежуточная значимая конструкция.
			if previous != nil {
				second := v147Previous(lines, previous)
				name, _, _ := v147Name(second)
				if name == "question" && !v147Block(second) {
					continue
				}
			}
		}
		if _, err := findings.Add(number, start, number, end, ""); err != nil {
			return err
		}
	}
	return nil
}

// v147AfterMatchingSeparator самостоятельно распознаёт область V166:
// непосредственный answer после единственного собственного разделителя.
// ordering и вложенные ответы в это исключение не входят.
func v147AfterMatchingSeparator(lines diagnostics.Lines, block, answer diagnostics.Line) bool {
	if parent, _ := answer.ParentLine(); parent != block.Number() {
		return false
	}
	count, separator := 0, 0
	for number := block.Number() + 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if line.NestingLevel() <= block.NestingLevel() {
			break
		}
		if parent, _ := line.ParentLine(); parent != block.Number() {
			continue
		}
		if line.Type() == "block-end" {
			break
		}
		if strings.Trim(line.Raw(), " \t") == "---" {
			count++
			separator = number
		}
	}
	return count == 1 && answer.Number() > separator
}

// v147Name возвращает начальный tag или имя в частично разобранной строке.
// Колонки Unicode начинаются с 1, конец исключён. Обычный текст не становится
// тегом по семантическому значению. nil и отсутствие объявления дают пустое имя.
func v147Name(line diagnostics.Line) (string, int, int) {
	if line == nil {
		return "", 0, 0
	}
	raw := []rune(line.Raw())
	start := 0
	for start < len(raw) && (raw[start] == ' ' || raw[start] == '\t') {
		start++
	}
	if start == len(raw) || raw[start] != '@' {
		return "", 0, 0
	}
	end := start + 1
	for end < len(raw) && !unicode.Is(unicode.White_Space, raw[end]) && raw[end] != '{' && raw[end] != '}' {
		end++
	}
	name := strings.ToLower(string(raw[start+1 : end]))
	erroneous := line.Type() == "invalid"
	for i := 0; i < line.ElementCount(); i++ {
		element, _ := line.Element(i)
		if element.Type() == "tag" && element.Start() == start+1 && element.End() == end+1 {
			if value, present := element.Value(); present && strings.EqualFold(value, name) {
				return name, start + 1, end + 1
			}
		}
		erroneous = erroneous || element.Type() == "unparsed"
	}
	if erroneous {
		return name, start + 1, end + 1
	}
	return "", 0, 0
}

// v147Block сообщает, сохранила ли модель открытие фигурного блока.
func v147Block(line diagnostics.Line) bool {
	if line == nil {
		return false
	}
	if line.Type() == "block-start" {
		return true
	}
	for i := 0; i < line.ElementCount(); i++ {
		element, _ := line.Element(i)
		if element.Type() == "block-open" {
			return true
		}
	}
	return false
}

// v147BlockParent возвращает ближайшего фигурного предка или nil у корня.
func v147BlockParent(lines diagnostics.Lines, line diagnostics.Line) diagnostics.Line {
	for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
		line, _ = lines.Line(parent)
		if v147Block(line) {
			return line
		}
	}
	return nil
}

// v147Literal исключает editor и multifill по всей цепочке предков.
// Запрещённый реальный answer внутри multifill также исключён как случай V142.
func v147Literal(lines diagnostics.Lines, line diagnostics.Line) bool {
	for block := v147BlockParent(lines, line); block != nil; block = v147BlockParent(lines, block) {
		name, _, _ := v147Name(block)
		if name == "editor" || name == "multifill" {
			return true
		}
	}
	return false
}

// v147SameLevel проверяет совпадение логического родителя и глубины.
func v147SameLevel(left, right diagnostics.Line) bool {
	lp, lf := left.ParentLine()
	rp, rf := right.ParentLine()
	return lp == rp && lf == rf && left.NestingLevel() == right.NestingLevel()
}

// v147QuestionContext разрешает вопрос в шаге, задании, этапе или ветви.
// Фигурный контейнер не пропускается; variants требует уже начавшейся ветви.
func v147QuestionContext(lines diagnostics.Lines, line diagnostics.Line) bool {
	structural, branch := false, false
	for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
		line, _ = lines.Line(parent)
		name, _, _ := v147Name(line)
		if v147Block(line) {
			return name == "variants" && branch
		}
		branch = branch || name == "variant"
		structural = structural || line.Type() == "heading" || name == "task" || name == "step" || branch
	}
	return structural
}

// v147EditorAllowed применяет места разрешения editor из п. 2.8.9.
func v147EditorAllowed(lines diagnostics.Lines, line diagnostics.Line) bool {
	if v147QuestionContext(lines, line) {
		return true
	}
	if block := v147BlockParent(lines, line); block != nil {
		name, _, _ := v147Name(block)
		return name == "variants"
	}
	if _, found := line.ParentLine(); found {
		return false
	}
	version := false
	for n := 1; n < line.Number(); n++ {
		previous, _ := lines.Line(n)
		if previous.Type() == "heading" {
			return false
		}
		name, _, _ := v147Name(previous)
		version = version || name == "dsl-version"
	}
	return version
}

// v147Previous возвращает соседнюю конструкцию на том же структурном уровне.
// nil означает границу или отсутствие соседа. Закрытый answer/multifill
// узнаётся целиком для специализированных исключений; иные блоки — барьер.
func v147Previous(lines diagnostics.Lines, current diagnostics.Line) diagnostics.Line {
	parent, _ := current.ParentLine()
	for n := current.Number() - 1; n > parent; n-- {
		line, _ := lines.Line(n)
		if line.Type() == "block-end" {
			line = v147BlockParent(lines, line)
			if line == nil || !v147SameLevel(line, current) {
				return nil
			}
			name, _, _ := v147Name(line)
			if name == "editor" && v147EditorAllowed(lines, line) {
				n = line.Number()
				continue
			}
			if name == "answer" || name == "multifill" {
				return line
			}
			return nil
		}
		if !v147SameLevel(line, current) {
			return nil
		}
		if strings.Trim(line.Raw(), " \t") == "" {
			continue
		}
		name, _, _ := v147Name(line)
		if name == "editor" && !v147Block(line) && v147EditorAllowed(lines, line) {
			continue
		}
		return line
	}
	return nil
}

// v147Item признаёт только непустую текстовую строку списка, не тег или границу.
func v147Item(line diagnostics.Line) bool {
	return line != nil && line.Type() == "content" && strings.Trim(line.Raw(), " \t") != ""
}

// v147BeforeSeparator проверяет число собственных разделителей всего блока.
// ordering допускает отсутствие разделителя. Вложенные блоки не дают разделителей.
// Raw распознаётся и при частичном разборе, как в области исключения V166.
func v147BeforeSeparator(lines diagnostics.Lines, block diagnostics.Line, answer int, ordering bool) bool {
	count, separator := 0, 0
	for n := block.Number() + 1; n <= lines.Len(); n++ {
		line, _ := lines.Line(n)
		inside := false
		for ancestor, found := line.ParentLine(); found; ancestor, found = line.ParentLine() {
			if ancestor == block.Number() {
				inside = true
				break
			}
			line, _ = lines.Line(ancestor)
		}
		if !inside {
			break
		}
		line, _ = lines.Line(n)
		if parent, _ := line.ParentLine(); parent != block.Number() {
			continue
		}
		if line.Type() == "block-end" {
			break
		}
		if strings.Trim(line.Raw(), " \t") == "---" {
			count++
			separator = n
		}
	}
	return count == 0 && ordering || count == 1 && answer < separator
}
