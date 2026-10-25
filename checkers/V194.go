package checkers

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// v194Material различает доказанную пустоту, наличие материала и неопределённость.
type v194Material uint8

const (
	v194Empty v194Material = iota
	v194Present
	v194Unknown
)

// v194Branch сохраняет исходные границы одной ветви, без состояния задания.
type v194Branch struct {
	start, boundary, container int
	related                    diagnostics.Element
}

// v194Document хранит локальные индексы готовой структуры одного документа.
// Нулевое закрытие отсутствует, -1 означает несколько закрытий одного блока.
type v194Document struct {
	lines     diagnostics.Lines
	parents   []int
	ends      map[int]int
	fragments []int
	material  map[int]v194Material
}

// registrationV194 регистрирует пустые ветви только для DSL 1.2.
func registrationV194(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code: "V194", Severity: diagnostics.SeverityError,
		Message:  "Ветвь @variant не содержит отображаемого материала.",
		Scope:    diagnostics.ScopeBlock,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV194,
	})
}

// checkV194 выдаёт по одному сообщению только для ветвей с доказанной пустотой.
func checkV194(lines diagnostics.Lines, findings diagnostics.Findings) error {
	doc := v194Index(lines)
	for _, branch := range doc.branches() {
		if branch.boundary == 0 || branch.related == nil || doc.scan(branch.start+1, branch.boundary, branch.container, false) != v194Empty {
			continue
		}
		opening, _ := lines.Line(branch.start)
		last, _ := lines.Line(branch.boundary - 1)
		tag := v194Element(opening, "tag")
		occurrence, err := findings.Add(branch.start, tag.Start(), last.Number(), utf8.RuneCountInString(last.Raw())+1, "")
		if err != nil {
			return err
		}
		if err := findings.AddRelated(occurrence, branch.boundary, branch.related.Start(), branch.boundary, branch.related.End()); err != nil {
			return err
		}
	}
	return nil
}

// v194Index индексирует родителей и закрытия без восстановления парсера.
func v194Index(lines diagnostics.Lines) *v194Document {
	doc := &v194Document{lines: lines, parents: make([]int, lines.Len()+1), ends: make(map[int]int), material: make(map[int]v194Material)}
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if parent, found := line.ParentLine(); found {
			ancestor, _ := lines.Line(parent)
			doc.parents[number] = doc.parents[parent]
			if v194Block(ancestor) {
				doc.parents[number] = parent
			}
			if line.Type() == "block-end" {
				if doc.ends[parent] != 0 {
					doc.ends[parent] = -1
				} else {
					doc.ends[parent] = number
				}
			}
		}
	}
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if v194Tag(line) == "fragment" && !doc.hidden(number, false) {
			doc.fragments = append(doc.fragments, number)
		}
	}
	return doc
}

// branches выделяет ветви по событиям одного фигурного контейнера.
// Неизвестное закрытие не заменяется концом файла; уже завершённые ветви сохраняются.
func (doc *v194Document) branches() []v194Branch {
	var branches []v194Branch
	active := make(map[int]int)
	for number := 1; number <= doc.lines.Len(); number++ {
		line, _ := doc.lines.Line(number)
		container := doc.parents[number]
		if container == 0 || doc.hidden(number, true) {
			continue
		}
		parent, _ := doc.lines.Line(container)
		if v194Tag(parent) != "variants" {
			continue
		}
		if line.Type() == "block-end" {
			if index, found := active[container]; found {
				if doc.ends[container] == number {
					branches[index].boundary, branches[index].related = number, v194Element(line, "block-close")
				}
				delete(active, container)
			}
			continue
		}
		if end := doc.ends[container]; end > 0 && number >= end {
			continue
		}
		if v194Tag(line) != "variant" || v194Block(line) {
			continue
		}
		if index, found := active[container]; found {
			branches[index].boundary, branches[index].related = number, v194Element(line, "tag")
		}
		active[container] = len(branches)
		branches = append(branches, v194Branch{start: number, container: container})
	}
	return branches
}

// hidden возвращает true для потомков литерального блока; definitions также
// исключает содержимое определений fragment из выделения отображаемых ветвей.
func (doc *v194Document) hidden(number int, definitions bool) bool {
	for parent := doc.parents[number]; parent != 0; parent = doc.parents[parent] {
		line, _ := doc.lines.Line(parent)
		name := v194Tag(line)
		if name == "editor" || name == "multifill" || definitions && name == "fragment" {
			return true
		}
	}
	return false
}

// scan ищет материал в исходном полуинтервале строк одного фигурного контейнера.
// Наличие отображаемой конструкции не зависит от проверок её оформления.
// Поддеревья исключённых и запрещённых блоков не становятся обычным текстом.
func (doc *v194Document) scan(first, boundary, container int, fragment bool) v194Material {
	state := v194Empty
	for number := first; number < boundary; number++ {
		if doc.parents[number] != container {
			continue
		}
		line, _ := doc.lines.Line(number)
		name := v194Tag(line)
		if v194Displayed(name, fragment) {
			return v194Present
		}
		switch name {
		case "editor", "fragment", "answer":
			// Эти конструкции и их тела самостоятельно не отображаются.
		case "include":
			included := v194Unknown
			if !fragment {
				included = doc.include(line)
			}
			if included == v194Present {
				return v194Present
			}
			if included == v194Unknown {
				state = v194Unknown
			}
		default:
			if name == "" && line.Type() == "content" && v194Text(line) {
				return v194Present
			}
			if line.Type() == "invalid" || v194Element(line, "unparsed") != nil || line.Type() == "block-end" {
				state = v194Unknown
			} else if name == "" && line.Type() != "blank" && strings.Trim(line.Raw(), " \t") != "" {
				state = v194Unknown
			}
		}
		if v194Block(line) {
			end := doc.ends[number]
			if end <= number || end >= boundary {
				state = v194Unknown
			} else {
				number = end
			}
		}
	}
	return state
}

// include разрешает только однострочную вставку с полным допустимым ID.
// Вложенные include в fragment не разворачиваются: это запрещённая конструкция.
func (doc *v194Document) include(line diagnostics.Line) v194Material {
	id, valid := v194ID(line, false)
	if !valid {
		return v194Unknown
	}
	definition := 0
	for _, number := range doc.fragments {
		candidate, _ := doc.lines.Line(number)
		identifier := v194Element(candidate, "identifier")
		if identifier == nil {
			continue
		}
		value, _ := identifier.Value()
		if !strings.EqualFold(value, id) {
			continue
		}
		if definition != 0 {
			return v194Unknown
		}
		definition = number
	}
	if definition == 0 {
		return v194Unknown
	}
	if state, found := doc.material[definition]; found {
		return state
	}
	opening, _ := doc.lines.Line(definition)
	_, valid = v194ID(opening, true)
	end := doc.ends[definition]
	state := v194Unknown
	if valid && end > definition {
		state = doc.scan(definition+1, end, definition, true)
	}
	doc.material[definition] = state
	return state
}

// v194ID проверяет форму объявления и единственный ID, не нормализуя значение.
// false означает, что вставку нельзя однозначно интерпретировать.
func v194ID(line diagnostics.Line, fragment bool) (string, bool) {
	if v194Element(line, "unparsed") != nil || fragment && line.Type() != "block-start" || !fragment && line.Type() != "tag" {
		return "", false
	}
	tag, id := v194Element(line, "tag"), v194Element(line, "identifier")
	if tag == nil || id == nil {
		return "", false
	}
	value, _ := id.Value()
	if value == "" || !strings.EqualFold(id.Raw(), value) {
		return "", false
	}
	for _, char := range value {
		if !unicode.IsLetter(char) && !unicode.IsDigit(char) && char != '.' && char != '-' && char != '_' {
			return "", false
		}
	}
	raw := []rune(line.Raw())
	if id.Start() <= tag.End() || strings.Trim(string(raw[tag.End()-1:id.Start()-1]), " ") != "" {
		return "", false
	}
	after := id.End() - 1
	if fragment {
		opening := v194Element(line, "block-open")
		if opening == nil || opening.Start()-1 <= after || strings.Trim(string(raw[after:opening.Start()-1]), " ") != "" {
			return "", false
		}
		after = opening.End() - 1
	}
	return value, strings.Trim(string(raw[after:]), " \t") == ""
}

// v194Displayed определяет отображаемые конструкции из таблицы V194.
// Для тела fragment действует ограниченный состав по DSL 3.12.10.
func v194Displayed(name string, fragment bool) bool {
	switch name {
	case "media", "instr", "example", "wordlist", "table", "script", "text", "note", "alt", "hint":
		return true
	case "header", "task", "step", "key", "question", "multifill", "choice", "multichoice", "matching", "ordering":
		return !fragment
	}
	return false
}

// v194Text проверяет обычный текст без повторного декодирования и Unicode-trim.
func v194Text(line diagnostics.Line) bool {
	found := false
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() != "content" {
			continue
		}
		value, present := element.Value()
		if present {
			found = true
			if strings.Trim(value, " \t") != "" {
				return true
			}
		}
	}
	return !found && v194Element(line, "unparsed") == nil && strings.Trim(line.Raw(), " \t") != ""
}

// v194Tag читает только сохранённый начальный tag; unparsed не восстанавливает.
func v194Tag(line diagnostics.Line) string {
	tag := v194Element(line, "tag")
	if tag == nil {
		return ""
	}
	raw := []rune(line.Raw())
	first := 0
	for first < len(raw) && (raw[first] == ' ' || raw[first] == '\t') {
		first++
	}
	if first == len(raw) || raw[first] != '@' || tag.Start() != first+1 {
		return ""
	}
	name, _ := tag.Value()
	return strings.ToLower(name)
}

// v194Block возвращает true для сохранённого открытия фигурного блока.
func v194Block(line diagnostics.Line) bool {
	return line.Type() == "block-start" || v194Element(line, "block-open") != nil
}

// v194Element возвращает первый сохранённый элемент нужного типа либо nil.
func v194Element(line diagnostics.Line, kind string) diagnostics.Element {
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == kind {
			return element
		}
	}
	return nil
}
