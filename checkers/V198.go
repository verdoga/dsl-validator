package checkers

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV198 регистрирует сравнение полных ветвей только для DSL 1.2.
func registrationV198(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code: "V198", Severity: diagnostics.SeverityWarning, Scope: diagnostics.ScopeBlock,
		Message:  "У ветвей @variants совпадает всё содержимое.",
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV198,
	})
}

// v198Node сохраняет тип, значения, порядок и локальную связь с предыдущим узлом.
// link — расстояние до владельца в той же последовательности, ноль — нет связи.
// Исходные координаты намеренно не входят в сравниваемое представление.
type v198Node struct {
	kind   string
	fields []v198Field
	body   []v198Node
	link   int
}

// v198Field хранит тип и значение одного параметра или текстового фрагмента.
// Порядок полей значим; тип не смешивается со свободным текстом значения.
type v198Field struct {
	kind, value string
}

type v198Branch struct {
	first, last, column int
	name, key           string
}

// v198Scan хранит физические границы и определения только текущего документа.
type v198Scan struct {
	lines        diagnostics.Lines
	ends, owners map[int]int
	fragments    map[string][]int
}

// checkV198 группирует ветви независимо от имён и от результатов других проверок.
func checkV198(lines diagnostics.Lines, findings diagnostics.Findings) error {
	scan := v198Index(lines)
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if v198Name(line) != "variants" || line.Type() != "block-start" || scan.ends[number] <= number {
			continue
		}
		// variants внутри другого фигурного контейнера запрещён; литеральные
		// псевдообъявления не образуют отдельной области сравнения.
		fields, valid := v198Fields(line, "")
		if scan.owners[number] != 0 || !v198Clean(line) || !valid || len(fields) != 0 {
			continue
		}
		var branches []v198Branch
		for child := number + 1; child < scan.ends[number]; child++ {
			candidate, _ := lines.Line(child)
			if scan.owners[child] != number || v198Name(candidate) != "variant" {
				continue
			}
			if len(branches) != 0 {
				branches[len(branches)-1].last = child - 1
			}
			tag := v198Element(candidate, "tag")
			name := v198Element(candidate, "name")
			branch := v198Branch{first: child, last: scan.ends[number] - 1}
			if tag != nil {
				branch.column = tag.Start()
			}
			if name != nil {
				branch.name, _ = name.Value()
			}
			branches = append(branches, branch)
		}
		var groups [][]v198Branch
		indices := make(map[string]int)
		for _, branch := range branches {
			declaration, _ := lines.Line(branch.first)
			if declaration.Type() != "tag" || !v198Clean(declaration) || branch.column == 0 || !v198Nonblank(branch.name) {
				continue
			}
			nodes, valid := scan.sequence(branch.first+1, branch.last, "variant")
			if !valid || !v198Material(nodes) {
				continue
			}
			branch.key = v198Key(nodes)
			if index, found := indices[branch.key]; found {
				groups[index] = append(groups[index], branch)
			} else {
				indices[branch.key] = len(groups)
				groups = append(groups, []v198Branch{branch})
			}
		}
		for _, group := range groups {
			if len(group) < 2 {
				continue
			}
			names := make([]string, 0, len(group))
			for _, branch := range group {
				names = append(names, "«"+branch.name+"» (строка "+strconv.Itoa(branch.first)+")")
			}
			main := group[1]
			last, _ := lines.Line(main.last)
			occurrence, err := findings.Add(main.first, main.column, main.last, utf8.RuneCountInString(last.Raw())+1,
				"Совпадают ветви: "+strings.Join(names, ", ")+".")
			if err != nil {
				return err
			}
			for index, branch := range group {
				if index == 1 {
					continue
				}
				last, _ = lines.Line(branch.last)
				if err := findings.AddRelated(occurrence, branch.first, branch.column, branch.last, utf8.RuneCountInString(last.Raw())+1); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// v198Index использует готовые открытия и закрытия; заголовки не закрывают блок.
// Незавершённые или пересечённые блоки не получают доказанной границы.
func v198Index(lines diagnostics.Lines) v198Scan {
	scan := v198Scan{lines: lines, ends: make(map[int]int), owners: make(map[int]int), fragments: make(map[string][]int)}
	var stack []int
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if len(stack) > 0 {
			scan.owners[number] = stack[len(stack)-1]
		}
		if line.Type() == "block-end" {
			parent, found := line.ParentLine()
			if found && len(stack) > 0 && parent == stack[len(stack)-1] && v198Element(line, "block-close") != nil && v198Clean(line) {
				scan.ends[parent] = number
				stack = stack[:len(stack)-1]
			}
			continue
		}
		literal := false
		if owner := scan.owners[number]; owner != 0 {
			opening, _ := lines.Line(owner)
			literal = v198Name(opening) == "editor" || v198Name(opening) == "multifill"
		}
		if literal {
			continue
		}
		if line.Type() == "block-start" && v198Element(line, "block-open") != nil {
			stack = append(stack, number)
			if v198Name(line) == "fragment" {
				if id, valid := v198ID(line); valid {
					scan.fragments[id] = append(scan.fragments[id], number)
				}
			}
		}
	}
	return scan
}

// sequence читает уже разобранные конструкции одного контейнера. Вставки
// раскрываются в отдельном потоке, поэтому связи через их границы невозможны.
func (scan *v198Scan) sequence(first, last int, context string) ([]v198Node, bool) {
	var nodes []v198Node
	previous := -1
	task, step, speaking := false, false, false
	separated, separators, answers, hints := false, 0, 0, 0
	tableColumns := 0
	finishTask := func() {
		if step {
			nodes = append(nodes, v198Node{kind: "end-step"})
			step = false
		}
		if task {
			nodes = append(nodes, v198Node{kind: "end-task"})
			task = false
		}
	}
	for number := first; number <= last; number++ {
		line, _ := scan.lines.Line(number)
		name := v198Name(line)
		if context == "multifill" {
			if line.Type() != "content" && line.Type() != "blank" || !v198Clean(line) {
				return nil, false
			}
			// Литеральный контекст не отменяет запрет 4.4.11 и 4.4.18.
			// Проверяется Raw: снятое экранирование не запускает разбор снова.
			declaration := strings.ToLower(strings.Trim(line.Raw(), " \t"))
			if declaration == "@answer" || strings.HasPrefix(declaration, "@answer ") || strings.HasPrefix(declaration, "@answer\t") ||
				strings.HasPrefix(declaration, "@answer{") || strings.HasPrefix(declaration, "@answer}") {
				return nil, false
			}
			node, valid := v198Text(line, context)
			if !valid {
				return nil, false
			}
			nodes = append(nodes, node)
			continue
		}
		if line.Type() == "blank" {
			continue
		}
		if !v198Clean(line) {
			return nil, false
		}
		if name == "editor" && context == "variant" {
			if line.Type() == "block-start" {
				end := scan.ends[number]
				if end <= number || end > last {
					return nil, false
				}
				number = end
			} else if line.Type() != "tag" {
				return nil, false
			}
			continue
		}
		if name == "fragment" && context == "variant" {
			end := scan.ends[number]
			if line.Type() != "block-start" || end <= number || end > last {
				return nil, false
			}
			if _, valid := v198ID(line); !valid {
				return nil, false
			}
			number = end
			// Определение ничего не отображает, но остаётся значимой конструкцией.
			previous = -1
			continue
		}
		if name == "include" && context == "variant" {
			id, valid := v198ID(line)
			definitions := scan.fragments[id]
			if !valid || line.Type() != "tag" || len(definitions) != 1 {
				return nil, false
			}
			opening := definitions[0]
			end := scan.ends[opening]
			definition, _ := scan.lines.Line(opening)
			if end <= opening || !v198Clean(definition) || !scan.fragmentPlacement(opening) {
				return nil, false
			}
			body, valid := scan.sequence(opening+1, end-1, "fragment")
			if !valid || !v198Material(body) {
				return nil, false
			}
			nodes = append(nodes, body...)
			previous = -1
			continue
		}
		if line.Type() == "content" || line.Type() == "separator" {
			trimmed := strings.Trim(line.Raw(), " \t")
			if strings.HasPrefix(trimmed, "@") || strings.HasPrefix(trimmed, "#") {
				return nil, false
			}
			if strings.Trim(line.Raw(), " \t") == "---" && v198Contains("example table matching ordering", context) {
				separators++
				if separators > 1 {
					return nil, false
				}
				separated = true
				nodes = append(nodes, v198Node{kind: "separator"})
			} else {
				if line.Type() == "separator" {
					return nil, false
				}
				if context == "table" {
					columns, valid := v198TableColumns(line.Raw())
					if !valid || tableColumns != 0 && columns != tableColumns {
						return nil, false
					}
					tableColumns = columns
				}
				node, valid := v198Text(line, context)
				if !valid {
					return nil, false
				}
				if !v198Material([]v198Node{node}) {
					continue
				}
				nodes = append(nodes, node)
			}
			previous = len(nodes) - 1
			continue
		}
		if !v198Allowed(context, name) {
			return nil, false
		}
		if context == "variant" {
			if name == "key" && !task {
				return nil, false
			}
			switch name {
			case "task", "header", "endtask":
				if name == "endtask" && !task {
					return nil, false
				}
				finishTask()
				previous = -1
				if name == "endtask" {
					fields, valid := v198Fields(line, "")
					if line.Type() != "tag" || !valid || len(fields) != 0 {
						return nil, false
					}
					continue
				}
				if name == "task" {
					task, speaking = true, false
				}
			case "step":
				if !task {
					return nil, false
				}
				if step {
					nodes = append(nodes, v198Node{kind: "end-step"})
				}
				step = true
			case "speaking":
				if !task || speaking || previous < 0 || nodes[previous].kind != "task" {
					return nil, false
				}
				speaking = true
			}
		}
		node, end, valid := scan.construction(number, last)
		if !valid {
			return nil, false
		}
		index := len(nodes)
		prior := ""
		if previous >= 0 {
			prior = nodes[previous].kind
		}
		switch name {
		case "answer":
			answers++
			if context == "variant" && prior != "question" ||
				(context == "matching" || context == "ordering") && (separated || prior != "paragraph") ||
				context == "choice" && answers > 1 {
				return nil, false
			}
			if context == "variant" || context == "matching" || context == "ordering" {
				node.link = index - previous
			}
		case "hint":
			hints++
			if context == "instr" {
				if hints > 1 {
					return nil, false
				}
			} else if v198Contains("choice multichoice matching ordering", context) && (prior != "answer" || separated) {
				return nil, false
			}
			if prior == "answer" {
				node.link = index - previous
			}
		case "script":
			if prior == "media" && v198AudioVideo(nodes[previous]) {
				node.link = index - previous
			}
		}
		nodes = append(nodes, node)
		previous, number = len(nodes)-1, end
	}
	finishTask()
	if !v198Shape(nodes, context) {
		return nil, false
	}
	return nodes, true
}

// construction сохраняет параметры объявления и отдельно его полное тело.
// Однострочное текстовое содержимое становится тем же абзацем, что строка блока.
func (scan *v198Scan) construction(number, last int) (v198Node, int, bool) {
	line, _ := scan.lines.Line(number)
	name := v198Name(line)
	node := v198Node{kind: name}
	block := line.Type() == "block-start"
	if !block && line.Type() != "tag" {
		return node, number, false
	}
	if block && v198Contains("media header task step speaking endtask question include", name) ||
		!block && v198Contains("table script text key choice multichoice matching ordering", name) {
		return node, number, false
	}
	if block {
		end := scan.ends[number]
		if end <= number || end > last {
			return node, number, false
		}
		var valid bool
		node.fields, valid = v198Fields(line, "")
		if !valid || v198Contains("instr hint answer", name) && len(node.fields) != 0 {
			return node, number, false
		}
		node.body, valid = scan.sequence(number+1, end-1, name)
		if !valid || !v198Material(node.body) {
			return node, number, false
		}
		return node, end, true
	}
	if v198Contains("example instr note alt hint answer wordlist", name) {
		context := name
		if name == "wordlist" {
			context, node.kind = "wordlist-inline", "wordlist-inline"
		}
		text, valid := v198Text(line, context)
		if !valid || !v198Material([]v198Node{text}) {
			return node, number, false
		}
		node.body = []v198Node{text}
	} else {
		var valid bool
		node.fields, valid = v198Fields(line, "")
		if !valid || name != "multifill" && name != "speaking" && len(node.fields) == 0 {
			return node, number, false
		}
		if name == "speaking" && len(node.fields) != 0 {
			return node, number, false
		}
		if name == "media" {
			if v198Element(line, "media-type") == nil || v198Element(line, "source") == nil {
				return node, number, false
			}
			source := v198Element(line, "source")
			value, _ := source.Value()
			numeric := value != "" && !strings.HasPrefix(source.Raw(), "\"")
			for _, r := range value {
				numeric = numeric && r >= '0' && r <= '9'
			}
			if numeric {
				node.fields = append(node.fields, v198Field{kind: "source-mode", value: "lookup-number"})
			} else {
				node.fields = append(node.fields, v198Field{kind: "source-mode", value: "path"})
			}
		}
	}
	return node, number, true
}

// v198Shape проверяет структурные связи и разделители без обращения к чужим
// диагностикам. Значения ответов не нормализуются правилами автопроверки.
func v198Shape(nodes []v198Node, context string) bool {
	if context == "instr" {
		for _, node := range nodes {
			if node.kind == "paragraph" && v198Material([]v198Node{node}) {
				return true
			}
		}
		return false
	}
	if !v198Contains("choice multichoice matching ordering example table", context) {
		return true
	}
	upper, lower, answers, separator := 0, 0, 0, false
	for _, node := range nodes {
		switch node.kind {
		case "separator":
			separator = true
		case "paragraph":
			if separator {
				lower++
			} else {
				upper++
			}
		case "answer":
			answers++
			if context == "choice" || context == "multichoice" {
				if context == "choice" && len(node.body) != 1 {
					return false
				}
				upper += len(node.body)
			}
		}
	}
	switch context {
	case "choice", "multichoice":
		return upper >= 2
	case "matching":
		return separator && upper >= 1 && lower >= 2 && (answers == 0 || answers == upper)
	case "ordering":
		return upper >= 2 && (!separator || lower >= 2) && (answers == 0 || answers == upper)
	default:
		return !separator || upper > 0 && lower > 0
	}
}

// v198Allowed задаёт разрешённые непосредственные вложения DSL 1.2.
func v198Allowed(context, name string) bool {
	switch context {
	case "variant":
		return v198Contains("header task speaking step endtask media example wordlist table script text key instr note alt hint question answer multifill choice multichoice matching ordering", name)
	case "fragment":
		return v198Contains("media example wordlist table script text instr note alt hint", name)
	case "instr":
		return name == "hint"
	case "choice", "multichoice", "matching", "ordering":
		return name == "answer" || name == "hint"
	default:
		return false
	}
}

// fragmentPlacement исключает определения внутри запрещённого фигурного блока.
func (scan *v198Scan) fragmentPlacement(number int) bool {
	heading := false
	for i := 1; i < number; i++ {
		line, _ := scan.lines.Line(i)
		if element := v198Element(line, "heading-level"); element != nil {
			value, _ := element.Value()
			heading = heading || value == "1" && scan.owners[i] == 0
		}
	}
	if !heading {
		return false
	}
	owner := scan.owners[number]
	if owner == 0 {
		return true
	}
	line, _ := scan.lines.Line(owner)
	if v198Name(line) != "variants" {
		return false
	}
	for i := owner + 1; i < number; i++ {
		line, _ = scan.lines.Line(i)
		if scan.owners[i] == owner && v198Name(line) == "variant" {
			return true
		}
	}
	return false
}

// v198Text представляет одну физическую строку содержимого отдельным абзацем.
func v198Text(line diagnostics.Line, context string) (v198Node, bool) {
	fields, valid := v198Fields(line, context)
	node := v198Node{kind: "paragraph", fields: fields}
	if !valid || len(fields) == 0 && line.Type() != "blank" {
		return node, false
	}
	for _, field := range fields {
		if !v198Contains("content placeholder syntax", field.kind) {
			return node, false
		}
	}
	return node, true
}

// v198Fields одинаково собирает параметры и содержимое из готовых Value.
// Соседние части текста объединяются с исходными промежутками; плейсхолдеры
// остаются отдельными полями. Raw уточняет структуру, но не декодирует Value.
// Пустой context обозначает параметры объявления, а не содержимое блока.
func v198Fields(line diagnostics.Line, context string) ([]v198Field, bool) {
	var fields []v198Field
	raw := []rune(line.Raw())
	end, first := 0, 0
	previousText := false
	appendText := func(text string) {
		if text == "" {
			return
		}
		if len(fields) > 0 && fields[len(fields)-1].kind == "content" {
			fields[len(fields)-1].value += text
		} else {
			fields = append(fields, v198Field{kind: "content", value: text})
		}
	}
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		kind := element.Type()
		if v198Contains("tag block-open block-close", kind) {
			previousText = false
			continue
		}
		text := kind == "content" || kind == "placeholder"
		if text && first == 0 {
			first = element.Start()
		}
		if previousText && text && element.Start()-1 > end {
			appendText(string(raw[end : element.Start()-1]))
		}
		value, present := element.Value()
		if kind == "placeholder" {
			answers, valid := v198Placeholder(element.Raw())
			if !valid || context != "multifill" && len(answers) > 0 {
				return nil, false
			}
			value = v198Key([]v198Node{{kind: "answers", fields: answers}})
		} else {
			if !present {
				return nil, false
			}
		}
		if kind == "content" {
			appendText(value)
		} else {
			fields = append(fields, v198Field{kind: kind, value: value})
		}
		previousText, end = text, element.End()-1
	}
	if first != 0 && v198Contains("table text wordlist-inline", context) {
		lineStart := strings.Trim(string(raw[:first-1]), " \t") == ""
		source := string(raw[first-1 : end])
		if context == "text" {
			source = strings.Trim(source, " \t")
			if !v198TextHTML(source, fields, lineStart) {
				return nil, false
			}
		}
		fields = append(fields, v198Field{kind: "syntax", value: v198Syntax(source, context, lineStart)})
	}
	return fields, true
}

// v198TextHTML проверяет HTML всей физической строки, сохраняя типы элементов.
// Обычный плейсхолдер занимает пять позиций; ответы в @text уже исключены.
// Готовые значения content меняются лишь в регистре имён HTML-тегов.
func v198TextHTML(raw string, fields []v198Field, lineStart bool) bool {
	var text strings.Builder
	for _, field := range fields {
		switch field.kind {
		case "content":
			text.WriteString(field.value)
		case "placeholder":
			text.WriteString("_____")
		default:
			return false
		}
	}
	value, valid := v198HTML(raw, text.String(), lineStart)
	if !valid {
		return false
	}
	decoded, position := []rune(value), 0
	for index, field := range fields {
		if field.kind == "content" {
			length := utf8.RuneCountInString(field.value)
			fields[index].value = string(decoded[position : position+length])
			position += length
		} else {
			position += 5
		}
	}
	return true
}

// v198Placeholder сохраняет каждую группу ответа отдельно. Value одного
// placeholder не гарантирует представления всех альтернатив; Raw содержит их все.
func v198Placeholder(raw string) ([]v198Field, bool) {
	if !strings.HasPrefix(raw, "_____") {
		return nil, false
	}
	runes := []rune(raw)
	var answers []v198Field
	for i := 5; i < len(runes); {
		if runes[i] != '{' {
			return nil, false
		}
		i++
		var value strings.Builder
		for i < len(runes) && runes[i] != '}' {
			if runes[i] == '{' {
				return nil, false
			}
			if runes[i] == '\\' && i+1 < len(runes) && strings.ContainsRune("\\{}", runes[i+1]) {
				i++
			}
			value.WriteRune(runes[i])
			i++
		}
		if i == len(runes) || strings.IndexFunc(value.String(), func(r rune) bool { return !unicode.IsSpace(r) }) < 0 {
			return nil, false
		}
		answers = append(answers, v198Field{kind: "answer", value: strings.Trim(value.String(), " \t")})
		i++
	}
	return answers, true
}

// v198HTML проверяет вложенность поддерживаемых маркеров и канонизирует только
// имена служебных HTML-тегов. Экранированные углы остаются свободным текстом.
// Raw сопоставляется с готовым Value посимвольно без повторного декодирования;
// несовпадение исключает ветвь, а не разрешает перезапись свободного текста.
func v198HTML(raw, value string, lineStart bool) (string, bool) {
	runes, decoded := []rune(raw), []rune(value)
	position := 0
	var stack []string
	for i := 0; i < len(runes); i++ {
		escaped := v198Escaped(runes, i, "text", lineStart)
		if escaped {
			i++
		}
		if escaped || runes[i] != '<' {
			if position >= len(decoded) || decoded[position] != runes[i] {
				return "", false
			}
			position++
			continue
		}
		end := i + 1
		for end < len(runes) && runes[end] != '>' {
			end++
		}
		if end == len(runes) {
			return "", false
		}
		marker := strings.ToLower(string(runes[i : end+1]))
		if !v198Contains("<strong> </strong> <em> </em> <u> </u> <br>", marker) {
			return "", false
		}
		if marker == "<br>" {
			if strings.Trim(raw, " \t") != string(runes[i:end+1]) {
				return "", false
			}
		} else if strings.HasPrefix(marker, "</") {
			name := marker[2 : len(marker)-1]
			if len(stack) == 0 || stack[len(stack)-1] != name {
				return "", false
			}
			stack = stack[:len(stack)-1]
		} else {
			stack = append(stack, marker[1:len(marker)-1])
		}
		if position+len(marker) > len(decoded) || !strings.EqualFold(string(decoded[position:position+len(marker)]), marker) {
			return "", false
		}
		copy(decoded[position:position+len(marker)], []rune(marker))
		position += len(marker)
		i = end
	}
	return string(decoded), len(stack) == 0 && position == len(decoded)
}

// v198Escaped отмечает снятие одной косой черты по правилам текущего текста.
// lineStart означает начало физической строки после удаления отступов, а не
// начало элемента. Позиционные ограничения углов таблицы уточняет v198Syntax.
func v198Escaped(raw []rune, index int, context string, lineStart bool) bool {
	if raw[index] != '\\' || index+1 >= len(raw) {
		return false
	}
	next := raw[index+1]
	if strings.ContainsRune("\\{}", next) {
		return true
	}
	if next == '@' || next == '#' {
		return lineStart && index == 0 && context != "wordlist-inline"
	}
	if next == '_' {
		return index+2 < len(raw) && raw[index+2] == '_'
	}
	return (context == "text" || context == "table") && (next == '<' || next == '>') ||
		context == "table" && next == '|' || context == "wordlist-inline" && next == ';'
}

// v198Syntax различает литеральные и служебные знаки в координатах Value,
// учитывая только контекстное экранирование Raw по пунктам 1.21–1.22.
func v198Syntax(raw, context string, lineStart bool) string {
	var result strings.Builder
	position := 0
	runes := []rune(raw)
	cellStart, split := true, false
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		escaped := v198Escaped(runes, i, context, lineStart)
		if escaped && context == "table" && !split {
			if runes[i+1] == '<' {
				escaped = cellStart
			} else if runes[i+1] == '>' {
				next := i + 2
				for next < len(runes) && (runes[next] == ' ' || runes[next] == '\t') {
					next++
				}
				escaped = next == len(runes) || runes[next] == '|'
			}
		}
		if escaped {
			i++
			position++
			cellStart = false
			continue
		}
		if context == "table" && strings.ContainsRune("|<>", r) || context == "text" && strings.ContainsRune("<>", r) || context == "wordlist-inline" && r == ';' {
			result.WriteString(strconv.Itoa(position))
			result.WriteRune(r)
		}
		if context == "table" && r != ' ' && r != '\t' {
			if r == '|' && !split {
				cellStart = true
			} else {
				if r == '<' && cellStart {
					split = true
				} else if r == '>' && split {
					split = false
				}
				cellStart = false
			}
		}
		position++
	}
	return result.String()
}

// v198TableColumns проверяет границы внешних и разделённых ячеек, не меняя текст.
func v198TableColumns(raw string) (int, bool) {
	columns, parts := 1, 0
	start, split, closed := true, false, false
	runes := []rune(raw)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r == '\\' && i+1 < len(runes) && strings.ContainsRune("\\{}|<>", runes[i+1]) {
			if closed {
				return 0, false
			}
			i++
			start = false
			continue
		}
		if r == ' ' || r == '\t' {
			continue
		}
		if r == '|' {
			if split {
				parts++
			} else {
				columns++
				start, closed = true, false
			}
			continue
		}
		if closed {
			return 0, false
		}
		if r == '<' {
			if split {
				return 0, false
			}
			if start {
				split, parts = true, 1
			}
		} else if r == '>' && split {
			if parts < 2 {
				return 0, false
			}
			split, closed = false, true
		}
		start = false
	}
	return columns, !split
}

// v198Key кодирует полное представление без хеширования и неоднозначных стыков.
func v198Key(nodes []v198Node) string {
	var out strings.Builder
	for _, node := range nodes {
		out.WriteString(strconv.Quote(node.kind))
		out.WriteString(strconv.Itoa(node.link))
		out.WriteByte('[')
		for _, field := range node.fields {
			out.WriteString(strconv.Quote(field.kind))
			out.WriteString(strconv.Quote(field.value))
		}
		out.WriteString("]{")
		out.WriteString(v198Key(node.body))
		out.WriteByte('}')
	}
	return out.String()
}

// v198Material не считает служебные границы и определения отображаемым материалом.
func v198Material(nodes []v198Node) bool {
	for _, node := range nodes {
		if v198Contains("end-task end-step speaking separator", node.kind) {
			continue
		}
		if node.kind != "paragraph" {
			return true
		}
		for _, field := range node.fields {
			if field.kind != "syntax" && v198Nonblank(field.value) {
				return true
			}
		}
	}
	return false
}

func v198AudioVideo(node v198Node) bool {
	for _, field := range node.fields {
		if field.kind == "media-type" && v198Contains("audio video", field.value) {
			return true
		}
	}
	return false
}

func v198ID(line diagnostics.Line) (string, bool) {
	fields, valid := v198Fields(line, "")
	if !valid || len(fields) != 1 || fields[0].kind != "identifier" || fields[0].value == "" {
		return "", false
	}
	for _, r := range fields[0].value {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '.' && r != '-' && r != '_' {
			return "", false
		}
	}
	return strings.ToLower(fields[0].value), true
}

func v198Clean(line diagnostics.Line) bool {
	if line.Type() == "invalid" || line.Type() == "heading" {
		return false
	}
	for i := 0; i < line.ElementCount(); i++ {
		element, _ := line.Element(i)
		if element.Type() == "unparsed" {
			return false
		}
		if _, present := element.Value(); !present && element.Type() != "placeholder" {
			return false
		}
	}
	return true
}

func v198Element(line diagnostics.Line, kind string) diagnostics.Element {
	var result diagnostics.Element
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == kind {
			if result != nil {
				return nil
			}
			result = element
		}
	}
	return result
}

func v198Name(line diagnostics.Line) string {
	if tag := v198Element(line, "tag"); tag != nil {
		value, _ := tag.Value()
		return strings.ToLower(value)
	}
	return ""
}

func v198Contains(list, value string) bool {
	return value != "" && strings.Contains(" "+list+" ", " "+value+" ")
}

func v198Nonblank(value string) bool {
	return strings.Trim(value, " \t") != ""
}
