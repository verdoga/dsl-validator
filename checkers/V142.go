package checkers

import (
	"strings"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV142 регистрирует запрет answer внутри и после multifill в DSL 1.2.
func registrationV142(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code: "V142", Severity: diagnostics.SeverityError,
		Message:  "@answer размещён внутри @multifill или непосредственно после него.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV142,
	})
}

// checkV142 использует готовые границы блоков, не интерпретируя плейсхолдеры.
// Каждая строка внутри проверяется один раз; поиск после завершается на первой
// значимой конструкции, поэтому один answer не связывается с несколькими полями.
func checkV142(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		parentNumber, hasParent := line.ParentLine()
		var parent diagnostics.Line
		if hasParent {
			parent, _ = lines.Line(parentNumber)
		}
		if parent != nil && parent.Type() == "block-start" && v142Multifill(lines, parent) {
			if start, end := v142InitialAnswer(line.Raw()); start != 0 {
				if err := v142Report(findings, line, start, end, parent, "Внутри блока @multifill."); err != nil {
					return err
				}
				continue
			}
		}
		var field diagnostics.Line
		if line.Type() == "tag" && v142Multifill(lines, line) {
			field = line
		} else if line.Type() == "block-end" && parent != nil && v142Multifill(lines, parent) {
			field = parent
		}
		if field == nil {
			continue
		}
		answer := v142Following(lines, field, number+1)
		if answer == nil {
			continue
		}
		tag := v142Tag(answer, "answer")
		if err := v142Report(findings, answer, tag.Start(), tag.End(), field, "Непосредственно после @multifill."); err != nil {
			return err
		}
	}
	return nil
}

// v142Tag возвращает распознанное объявление, включая частично ошибочное.
// Строки обычного содержимого не становятся объявлениями по семантическому Value.
func v142Tag(line diagnostics.Line, name string) diagnostics.Element {
	if line.Type() != "tag" && line.Type() != "block-start" && line.Type() != "invalid" {
		return nil
	}
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		value, _ := element.Value()
		if element.Type() == "tag" && strings.EqualFold(value, name) {
			return element
		}
	}
	return nil
}

// v142Multifill отличает обе формы поля от псевдотегов литерального содержимого.
// true означает реальное поле; неправильное размещение в иных блоках не исключает его.
func v142Multifill(lines diagnostics.Lines, line diagnostics.Line) bool {
	if (line.Type() != "tag" && line.Type() != "block-start") || v142Tag(line, "multifill") == nil {
		return false
	}
	for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
		line, _ = lines.Line(parent)
		if line.Type() == "block-start" && (v142Tag(line, "editor") != nil || v142Tag(line, "multifill") != nil) {
			return false
		}
	}
	return true
}

// v142InitialAnswer возвращает диапазон начального @answer в Unicode-колонках.
// Начало 0 означает отсутствие; конец исключается. Raw не декодируется.
func v142InitialAnswer(raw string) (start, end int) {
	text := []rune(raw)
	first := 0
	for first < len(text) && (text[first] == ' ' || text[first] == '\t') {
		first++
	}
	last := first + len("@answer")
	if last > len(text) || !strings.EqualFold(string(text[first:last]), "@answer") {
		return 0, 0
	}
	if last < len(text) && text[last] != '{' && text[last] != '}' && !unicode.IsSpace(text[last]) {
		return 0, 0
	}
	return first + 1, last + 1
}

// v142Following ищет answer после поля, пропуская лишь пустые строки и editor.
// Смена родителя или глубины, обычный текст и любой другой тег останавливают поиск.
func v142Following(lines diagnostics.Lines, field diagnostics.Line, number int) diagnostics.Line {
	for ; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if !v142SameContext(field, line) {
			return nil
		}
		if line.Type() == "blank" {
			continue
		}
		if v142Comment(lines, line) {
			if line.Type() == "block-start" {
				closing := v142CommentEnd(lines, line)
				if closing == 0 {
					return nil
				}
				number = closing
			}
			continue
		}
		if v142Tag(line, "answer") != nil {
			return line
		}
		return nil
	}
	return nil
}

// v142SameContext сравнивает родителя и логическую глубину двух конструкций.
func v142SameContext(left, right diagnostics.Line) bool {
	leftParent, leftFound := left.ParentLine()
	rightParent, rightFound := right.ParentLine()
	return leftFound == rightFound && leftParent == rightParent && left.NestingLevel() == right.NestingLevel()
}

// v142Comment разрешает пропуск разобранного editor лишь в структурной области.
// Среди фигурных предков допустимы только variants; непустота однострочного
// комментария проверяется по его текстовому значению, неизвестная форма не пропускается.
func v142Comment(lines diagnostics.Lines, line diagnostics.Line) bool {
	if (line.Type() != "tag" && line.Type() != "block-start") || v142Tag(line, "editor") == nil {
		return false
	}
	nonempty := false
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		switch element.Type() {
		case "tag", "block-open":
		case "content":
			if line.Type() == "block-start" {
				return false
			}
			value, _ := element.Value()
			nonempty = nonempty || strings.Trim(value, " \t") != ""
		default:
			return false
		}
	}
	if line.Type() == "tag" && !nonempty {
		return false
	}
	for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
		line, _ = lines.Line(parent)
		if line.Type() == "block-start" && v142Tag(line, "variants") == nil {
			return false
		}
	}
	return true
}

// v142CommentEnd находит закрытие непустого editor по связям готовой модели.
// 0 означает незакрытый или неразобранный комментарий; внешний контейнер не пересекается.
func v142CommentEnd(lines diagnostics.Lines, opening diagnostics.Line) int {
	nonempty := false
	for number := opening.Number() + 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		parent, _ := line.ParentLine()
		if parent != opening.Number() {
			return 0
		}
		if line.Type() == "block-end" {
			if nonempty {
				return number
			}
			return 0
		}
		if line.Type() != "content" && line.Type() != "blank" {
			return 0
		}
		nonempty = nonempty || strings.Trim(line.Raw(), " \t") != ""
	}
	return 0
}

// v142Report выдаёт одно сообщение с исходным диапазоном объявления поля.
func v142Report(findings diagnostics.Findings, answer diagnostics.Line, start, end int, field diagnostics.Line, detail string) error {
	occurrence, err := findings.Add(answer.Number(), start, answer.Number(), end, detail)
	if err != nil {
		return err
	}
	tag := v142Tag(field, "multifill")
	return findings.AddRelated(occurrence, field.Number(), tag.Start(), field.Number(), tag.End())
}
