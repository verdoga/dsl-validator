package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV108 регистрирует рекомендацию о соседстве расшифровки и медиа.
func registrationV108(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V108",
		Severity: diagnostics.SeverityRecommendation,
		Message:  "Перед @script нет соседнего @media audio или @media video.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV108,
	})
}

// checkV108 проверяет определения script один раз, не разворачивая include.
func checkV108(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if line.Type() != "block-start" || !v108Named(line, "script") ||
			v108Element(line, "block-open") == nil || v108Literal(lines, line) {
			continue
		}
		if v108Adjacent(lines, line) {
			continue
		}
		tag := v108Element(line, "tag")
		if _, err := findings.Add(number, tag.Start(), number, tag.End(),
			"Если это расшифровка записи, разместите её следующей значимой конструкцией после соответствующего @media audio или @media video"); err != nil {
			return err
		}
	}
	return nil
}

// v108Adjacent ищет медиа назад до первой значимой конструкции или родителя.
// Только разрешённый комментарий целиком может перескочить границы блока.
func v108Adjacent(lines diagnostics.Lines, script diagnostics.Line) bool {
	parent, _ := script.ParentLine()
	for number := script.Number() - 1; number > parent; number-- {
		line, _ := lines.Line(number)
		if strings.Trim(line.Raw(), " \t") == "" {
			continue
		}
		if line.Type() == "block-end" {
			opening := v108BlockParent(lines, line)
			if opening == nil || !v108Comment(lines, opening, script) {
				return false
			}
			number = opening.Number()
			continue
		}
		if line.Type() == "tag" && v108Comment(lines, line, script) {
			continue
		}
		if line.Type() != "tag" || !v108Named(line, "media") || !v108SameLevel(line, script) {
			return false
		}
		kind, source := v108Element(line, "media-type"), v108Element(line, "source")
		if kind == nil || source == nil {
			return false
		}
		mediaType, _ := kind.Value()
		path, present := source.Value()
		return present && path != "" && (strings.EqualFold(mediaType, "audio") || strings.EqualFold(mediaType, "video"))
	}
	return false
}

// v108Comment разрешает пропуск editor только в том же структурном окружении.
// Из фигурных контейнеров editor разрешён лишь в variants; fragment и любые
// контентные/интерактивные блоки не получают исключения для соседства.
func v108Comment(lines diagnostics.Lines, line, script diagnostics.Line) bool {
	if !v108Named(line, "editor") || !v108SameLevel(line, script) {
		return false
	}
	for block := v108BlockParent(lines, line); block != nil; block = v108BlockParent(lines, block) {
		if !v108Named(block, "variants") {
			return false
		}
	}
	return true
}

// v108Literal исключает псевдотеги на любой глубине литерального контейнера.
func v108Literal(lines diagnostics.Lines, line diagnostics.Line) bool {
	for block := v108BlockParent(lines, line); block != nil; block = v108BlockParent(lines, block) {
		if v108Named(block, "editor") || v108Named(block, "multifill") {
			return true
		}
	}
	return false
}

// v108BlockParent возвращает ближайшего фигурного предка или nil у корня.
func v108BlockParent(lines diagnostics.Lines, line diagnostics.Line) diagnostics.Line {
	for {
		parent, found := line.ParentLine()
		if !found {
			return nil
		}
		line, _ = lines.Line(parent)
		if line.Type() == "block-start" {
			return line
		}
	}
}

// v108SameLevel сравнивает родителя и логическую глубину объявлений.
func v108SameLevel(left, right diagnostics.Line) bool {
	leftParent, leftHasParent := left.ParentLine()
	rightParent, rightHasParent := right.ParentLine()
	return leftParent == rightParent && leftHasParent == rightHasParent && left.NestingLevel() == right.NestingLevel()
}

// v108Named сравнивает распознанное имя тега без учёта регистра.
func v108Named(line diagnostics.Line, name string) bool {
	tag := v108Element(line, "tag")
	if tag == nil {
		return false
	}
	value, present := tag.Value()
	return present && strings.EqualFold(value, name)
}

// v108Element возвращает существующий элемент без восстановления из Raw.
func v108Element(line diagnostics.Line, kind string) diagnostics.Element {
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == kind {
			return element
		}
	}
	return nil
}
