package checkers

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV197 регистрирует редакторскую проверку общего материала ветвей.
func registrationV197(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code: "V197", Severity: diagnostics.SeverityWarning, Scope: diagnostics.ScopeBlock,
		Message:  "Одинаковый материал повторяется во всех ветвях @variants.",
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}, Check: checkV197,
	})
}

// v197Range хранит исходные координаты с единицы, правая граница исключена.
type v197Range [4]int
type v197Unit struct {
	signature string
	span      v197Range
}
type v197Document struct {
	lines     diagnostics.Lines
	ends      map[int]int
	closed    map[int]bool
	fragments map[string][]int
}

func checkV197(lines diagnostics.Lines, findings diagnostics.Findings) error {
	doc := v197Document{lines: lines, ends: make(map[int]int), closed: make(map[int]bool), fragments: make(map[string][]int)}
	for n := 1; n <= lines.Len(); n++ {
		line, _ := lines.Line(n)
		if line.Type() == "block-start" {
			doc.ends[n] = n
		}
		for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
			line, _ = lines.Line(parent)
			if line.Type() == "block-start" {
				doc.ends[parent] = n
			}
		}
		line, _ = lines.Line(n)
		if line.Type() == "block-end" {
			if parent, found := line.ParentLine(); found {
				doc.closed[parent] = true
			}
		}
		if line.Type() == "block-start" && v197Tag(line) == "fragment" && doc.displayContext(line) {
			if id := v197ID(line); id != "" {
				doc.fragments[strings.ToLower(id)] = append(doc.fragments[strings.ToLower(id)], n)
			}
		}
	}
	for n := 1; n <= lines.Len(); n++ {
		line, _ := lines.Line(n)
		if line.Type() != "block-start" || v197Tag(line) != "variants" || !doc.displayContext(line) {
			continue
		}
		branches, valid := doc.branches(n)
		if !valid || len(branches) < 2 {
			continue
		}
		units := make([][]v197Unit, len(branches))
		for i, start := range branches {
			end := doc.ends[n] + 1
			if i+1 < len(branches) {
				end = branches[i+1]
			}
			units[i], _ = doc.collect(start+1, end, n, false)
		}
		seen := make(map[string]bool)
		for _, candidate := range units[0] {
			if seen[candidate.signature] {
				continue
			}
			seen[candidate.signature] = true
			var occurrences []v197Range
			var primary v197Range
			common := true
			for branch, material := range units {
				found := false
				for _, unit := range material {
					if unit.signature != candidate.signature {
						continue
					}
					if branch == 1 && !found {
						primary = unit.span
					}
					found = true
					occurrences = append(occurrences, unit.span)
				}
				if !found {
					common = false
					break
				}
			}
			if !common {
				continue
			}
			occurrence, err := findings.Add(primary[0], primary[1], primary[2], primary[3], "Проверьте, следует ли вынести этот материал перед блоком @variants.")
			if err != nil {
				return err
			}
			unique := map[v197Range]bool{primary: true}
			for _, span := range occurrences {
				if unique[span] {
					continue
				}
				unique[span] = true
				if err := findings.AddRelated(occurrence, span[0], span[1], span[2], span[3]); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// branches использует физические объявления только ближайшего variants.
func (doc *v197Document) branches(block int) ([]int, bool) {
	var branches []int
	for n := block + 1; n <= doc.ends[block]; n++ {
		line, _ := doc.lines.Line(n)
		if doc.outer(line) != block {
			continue
		}
		if line.Type() == "invalid" && strings.HasPrefix(strings.ToLower(strings.Trim(line.Raw(), " \t")), "@variant") {
			return nil, false
		}
		if v197Tag(line) == "variant" {
			if line.Type() != "tag" || v197Unparsed(line) {
				return nil, false
			}
			branches = append(branches, n)
		}
	}
	return branches, true
}

func (doc *v197Document) outer(line diagnostics.Line) int {
	for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
		line, _ = doc.lines.Line(parent)
		if line.Type() == "block-start" {
			return parent
		}
	}
	return 0
}

// displayContext исключает вложение в контент, интерактивные и литеральные блоки.
// Логические task, step и variant не создают фигурной границы.
func (doc *v197Document) displayContext(line diagnostics.Line) bool {
	branch, heading := false, false
	for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
		line, _ = doc.lines.Line(parent)
		if line.Type() == "heading" {
			heading = true
		}
		if v197Tag(line) == "variant" {
			branch = true
		}
		if line.Type() == "block-start" && (v197Tag(line) != "variants" || !branch) {
			return false
		}
	}
	return heading
}

func (doc *v197Document) accessible(line diagnostics.Line, root int) bool {
	for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
		if parent == root {
			return true
		}
		line, _ = doc.lines.Line(parent)
		if line.Type() == "block-start" {
			return false
		}
		switch v197Tag(line) {
		case "variant", "task", "step":
		default:
			return false
		}
	}
	return false
}

// collect собирает единицы, не объединяя текст через границы логических областей
// или include. В режиме fragment запрещённая конструкция делает вставку неясной.
func (doc *v197Document) collect(first, limit, root int, fragment bool) ([]v197Unit, bool) {
	var units []v197Unit
	var paragraphs []string
	var run v197Range
	runParent := -1
	flush := func() {
		if len(paragraphs) > 0 {
			units = append(units, v197Unit{v197Key(append([]string{"plain"}, paragraphs...)...), run})
		}
		paragraphs = nil
		runParent = -1
	}
	for n := first; n < limit; n++ {
		line, _ := doc.lines.Line(n)
		if line.Type() == "block-end" {
			continue
		}
		if !doc.accessible(line, root) {
			flush()
			continue
		}
		parent, _ := line.ParentLine()
		if strings.Trim(line.Raw(), " \t") == "" {
			continue
		}
		tag := v197Tag(line)
		if tag == "editor" && !fragment && doc.displayContext(line) {
			if runParent != -1 && runParent != parent {
				flush()
			}
			if line.Type() == "block-start" {
				n = doc.ends[n]
			}
			continue
		}
		if line.Type() == "content" && !v197Unparsed(line) {
			value := v197Text(line, 0, utf8.RuneCountInString(line.Raw()), "plain", true)
			if value == "" {
				continue
			}
			if runParent != parent {
				flush()
				run = doc.span(n, n)
				runParent = parent
			}
			paragraphs = append(paragraphs, value)
			run[2], run[3] = n, utf8.RuneCountInString(line.Raw())+1
			continue
		}
		flush()
		if fragment && tag != "instr" && !v197Material(tag) {
			return nil, false
		}
		if tag == "include" && line.Type() == "tag" && doc.displayContext(line) {
			definitions := doc.fragments[strings.ToLower(v197ID(line))]
			if len(definitions) == 1 && doc.closed[definitions[0]] {
				inserted, valid := doc.collect(definitions[0]+1, doc.ends[definitions[0]], definitions[0], true)
				if valid {
					for _, unit := range inserted {
						unit.span = doc.span(n, n)
						units = append(units, unit)
					}
				}
			}
		} else if v197Material(tag) && !(tag == "hint" && doc.answerBefore(line)) {
			if unit, valid := doc.material(line); valid {
				units = append(units, unit)
			} else if fragment {
				return nil, false
			}
		}
		if line.Type() == "block-start" {
			n = doc.ends[n]
		}
	}
	flush()
	return units, true
}

func (doc *v197Document) answerBefore(line diagnostics.Line) bool {
	parent, _ := line.ParentLine()
	for n := line.Number() - 1; n > parent; n-- {
		previous, _ := doc.lines.Line(n)
		if strings.Trim(previous.Raw(), " \t") == "" {
			continue
		}
		if previous.Type() == "block-end" {
			opening, found := previous.ParentLine()
			if !found {
				return false
			}
			previous, _ = doc.lines.Line(opening)
			n = opening
		}
		owner, _ := previous.ParentLine()
		if owner != parent {
			return false
		}
		if v197Tag(previous) == "editor" && doc.displayContext(previous) {
			continue
		}
		return v197Tag(previous) == "answer"
	}
	return false
}

func v197Material(tag string) bool {
	switch tag {
	case "text", "example", "wordlist", "table", "script", "note", "alt", "media", "hint":
		return true
	}
	return false
}

func (doc *v197Document) span(first, last int) v197Range {
	line, _ := doc.lines.Line(last)
	return v197Range{first, 1, last, utf8.RuneCountInString(line.Raw()) + 1}
}

// material сохраняет целый блок; вложенные DSL-конструкции не превращаются
// в самостоятельные совпадения. Названия не проходят обработку содержимого.
func (doc *v197Document) material(line diagnostics.Line) (v197Unit, bool) {
	tag := v197Tag(line)
	if v197Unparsed(line) {
		return v197Unit{}, false
	}
	if tag == "media" {
		return v197Media(line)
	}
	start, end, valid := v197Argument(line)
	if !valid {
		return v197Unit{}, false
	}
	parts := []string{tag, ""}
	last := line.Number()
	if line.Type() == "block-start" {
		if !doc.closed[last] {
			return v197Unit{}, false
		}
		last = doc.ends[last]
		parts[1] = v197Text(line, start, end, "title", false)
		if tag == "hint" && parts[1] != "" {
			return v197Unit{}, false
		}
		separators, rows, columns, before := 0, 0, -1, 0
		for n := line.Number() + 1; n < last; n++ {
			content, _ := doc.lines.Line(n)
			if strings.Trim(content.Raw(), " \t") == "" {
				continue
			}
			parent, _ := content.ParentLine()
			if parent != line.Number() || v197Unparsed(content) {
				return v197Unit{}, false
			}
			if (tag == "example" || tag == "table") && strings.Trim(content.Raw(), " \t") == "---" {
				separators++
				before = rows
				parts = append(parts, v197Key("separator"))
				continue
			}
			if content.Type() != "content" {
				return v197Unit{}, false
			}
			value, count, ok := v197Paragraph(content, 0, utf8.RuneCountInString(content.Raw()), tag, true)
			if !ok {
				return v197Unit{}, false
			}
			if tag == "table" {
				if columns != -1 && columns != count {
					return v197Unit{}, false
				}
				columns = count
			}
			parts = append(parts, value)
			rows++
		}
		if separators > 1 || separators == 1 && (before == 0 || before == rows) {
			return v197Unit{}, false
		}
	} else if line.Type() == "tag" {
		if tag == "text" || tag == "script" || tag == "table" {
			return v197Unit{}, false
		}
		value, _, ok := v197Paragraph(line, start, end, tag, false)
		if !ok || strings.Trim(string([]rune(line.Raw())[start:end]), " \t") == "" {
			return v197Unit{}, false
		}
		parts = append(parts, value)
	} else {
		return v197Unit{}, false
	}
	if len(parts) == 2 {
		return v197Unit{}, false
	}
	return v197Unit{v197Key(parts...), doc.span(line.Number(), last)}, true
}

// v197Key кодирует длины полей, исключая коллизии текстов со служебными знаками.
func v197Key(parts ...string) string {
	var key strings.Builder
	for _, part := range parts {
		key.WriteString(strconv.Itoa(len(part)))
		key.WriteByte(':')
		key.WriteString(part)
	}
	return key.String()
}

func v197Tag(line diagnostics.Line) string {
	if line.Type() != "tag" && line.Type() != "block-start" {
		return ""
	}
	var name string
	for i := 0; i < line.ElementCount(); i++ {
		element, _ := line.Element(i)
		if element.Type() == "tag" {
			if name != "" {
				return ""
			}
			name = strings.ToLower(strings.TrimPrefix(element.Raw(), "@"))
		}
	}
	if name != "" {
		return name
	}
	// Тип строки уже установлен парсером; восстанавливается лишь имя объявления.
	raw := strings.TrimLeft(line.Raw(), " \t")
	if !strings.HasPrefix(raw, "@") {
		return ""
	}
	end := 1
	for end < len(raw) && ((raw[end] >= 'A' && raw[end] <= 'Z') || (raw[end] >= 'a' && raw[end] <= 'z') || raw[end] == '-') {
		end++
	}
	if end == 1 || end < len(raw) && raw[end] != ' ' && raw[end] != '\t' {
		return ""
	}
	return strings.ToLower(raw[1:end])
}

func v197Unparsed(line diagnostics.Line) bool {
	for i := 0; i < line.ElementCount(); i++ {
		element, _ := line.Element(i)
		if element.Type() == "unparsed" {
			return true
		}
	}
	return line.Type() == "invalid"
}

// v197Argument возвращает границы параметров без фигурной скобки и пробелов.
func v197Argument(line diagnostics.Line) (int, int, bool) {
	raw := []rune(line.Raw())
	start, end := 0, len(raw)
	for start < end && (raw[start] == ' ' || raw[start] == '\t') {
		start++
	}
	if start == end || raw[start] != '@' {
		return 0, 0, false
	}
	for start < end && raw[start] != ' ' && raw[start] != '\t' {
		start++
	}
	for end > start && (raw[end-1] == ' ' || raw[end-1] == '\t') {
		end--
	}
	if line.Type() == "block-start" {
		if end <= start || raw[end-1] != '{' {
			return 0, 0, false
		}
		end--
	}
	for start < end && (raw[start] == ' ' || raw[start] == '\t') {
		start++
	}
	for end > start && (raw[end-1] == ' ' || raw[end-1] == '\t') {
		end--
	}
	return start, end, true
}

func v197ID(line diagnostics.Line) string {
	if v197Unparsed(line) {
		return ""
	}
	start, end, valid := v197Argument(line)
	if !valid {
		return ""
	}
	value := string([]rune(line.Raw())[start:end])
	if value == "" {
		return ""
	}
	for _, r := range value {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '.' && r != '-' && r != '_' {
			return ""
		}
	}
	return value
}

// v197Symbol сохраняет экранирование до распознавания структурных символов.
type v197Symbol struct {
	char    rune
	escaped bool
}

func v197Symbols(raw, context string, initial bool) []v197Symbol {
	var out []v197Symbol
	cellHasText, splitCell := false, false
	chars := []rune(strings.Trim(raw, " \t"))
	for i := 0; i < len(chars); i++ {
		if chars[i] == '\\' && i+1 < len(chars) {
			next := chars[i+1]
			allowed := strings.ContainsRune("\\{}", next)
			if context == "source" {
				allowed = next == '\\' || next == '"'
			}
			allowed = allowed || initial && i == 0 && (next == '@' || next == '#')
			allowed = allowed || context != "title" && context != "source" && next == '_' && i+2 < len(chars) && chars[i+2] == '_'
			allowed = allowed || (context == "example" || context == "table") && i == 0 && string(chars[i+1:]) == "---"
			allowed = allowed || context == "wordlist-inline" && next == ';'
			allowed = allowed || context == "text" && (next == '<' || next == '>')
			if context == "table" {
				lastInCell := true
				for j := i + 2; j < len(chars); j++ {
					if chars[j] == ' ' || chars[j] == '\t' {
						continue
					}
					lastInCell = chars[j] == '|'
					break
				}
				allowed = allowed || next == '|' || next == '<' && (splitCell || !cellHasText) || next == '>' && (splitCell || lastInCell)
			}
			allowed = allowed || context == "source" && next == '"'
			if allowed {
				out = append(out, v197Symbol{next, true})
				cellHasText = true
				i++
				continue
			}
		}
		out = append(out, v197Symbol{char: chars[i]})
		if context == "table" {
			if chars[i] == '|' && !splitCell {
				cellHasText = false
				continue
			}
			if chars[i] == '<' && !cellHasText {
				splitCell = true
			}
			if chars[i] == '>' && splitCell {
				splitCell = false
			}
			if chars[i] != ' ' && chars[i] != '\t' {
				cellHasText = true
			}
		}
	}
	return out
}

func v197SymbolText(symbols []v197Symbol) string {
	var text strings.Builder
	for _, symbol := range symbols {
		text.WriteRune(symbol.char)
	}
	return text.String()
}

// v197Text использует готовое значение только для соответствующего исходного
// диапазона. При его отсутствии декодирует исходник один раз в данном контексте.
func v197Text(line diagnostics.Line, start, end int, context string, initial bool) string {
	raw := []rune(line.Raw())
	for start < end && (raw[start] == ' ' || raw[start] == '\t') {
		start++
	}
	for end > start && (raw[end-1] == ' ' || raw[end-1] == '\t') {
		end--
	}
	for i := 0; i < line.ElementCount(); i++ {
		element, _ := line.Element(i)
		if element.Start() == start+1 && element.End() == end+1 && (element.Type() == "content" || element.Type() == "title" || element.Type() == "source") {
			if value, present := element.Value(); present {
				return value
			}
		}
	}
	return v197SymbolText(v197Symbols(string(raw[start:end]), context, initial))
}

func v197Paragraph(line diagnostics.Line, start, end int, tag string, initial bool) (string, int, bool) {
	raw := string([]rune(line.Raw())[start:end])
	if tag == "table" {
		return v197Table(v197Symbols(raw, "table", initial))
	}
	if tag == "text" {
		symbols := v197Symbols(raw, "text", initial)
		markup := false
		for _, symbol := range symbols {
			if symbol.char == '<' && !symbol.escaped {
				markup = true
			}
		}
		if !markup {
			return v197Key("html-paragraph", v197Key("text", v197Text(line, start, end, "text", initial))), 0, true
		}
		return v197HTML(symbols)
	}
	if tag == "wordlist" && !initial {
		symbols := v197Symbols(raw, "wordlist-inline", false)
		parts := []string{"inline-list"}
		first := 0
		for i, symbol := range symbols {
			if symbol.char == ';' && !symbol.escaped {
				parts = append(parts, v197SymbolText(symbols[first:i]))
				first = i + 1
			}
		}
		parts = append(parts, v197SymbolText(symbols[first:]))
		return v197Key(parts...), 0, true
	}
	return v197Key("paragraph", v197Text(line, start, end, tag, initial)), 0, true
}

// v197HTML различает литеральный текст и форматирование; декодированный текст
// не распознаётся повторно. Границы форматирования не выходят за абзац.
func v197HTML(symbols []v197Symbol) (string, int, bool) {
	parts := []string{"html-paragraph"}
	var stack []string
	first := 0
	for i := 0; i < len(symbols); i++ {
		if symbols[i].char != '<' || symbols[i].escaped {
			continue
		}
		if i > first {
			parts = append(parts, v197Key("text", v197SymbolText(symbols[first:i])))
		}
		end := i + 1
		for end < len(symbols) && (symbols[end].char != '>' || symbols[end].escaped) {
			end++
		}
		if end == len(symbols) {
			return "", 0, false
		}
		name := strings.ToLower(v197SymbolText(symbols[i+1 : end]))
		if name == "br" && i == 0 && end == len(symbols)-1 {
			return v197Key("break"), 0, true
		}
		closing := strings.HasPrefix(name, "/")
		base := strings.TrimPrefix(name, "/")
		if base != "strong" && base != "em" && base != "u" {
			return "", 0, false
		}
		if closing {
			if len(stack) == 0 || stack[len(stack)-1] != base {
				return "", 0, false
			}
			stack = stack[:len(stack)-1]
		} else {
			stack = append(stack, base)
		}
		parts = append(parts, v197Key("html", name))
		i = end
		first = end + 1
	}
	if len(stack) != 0 {
		return "", 0, false
	}
	if first < len(symbols) {
		parts = append(parts, v197Key("text", v197SymbolText(symbols[first:])))
	}
	return v197Key(parts...), 0, true
}

func v197TrimSymbols(symbols []v197Symbol) []v197Symbol {
	for len(symbols) > 0 && (symbols[0].char == ' ' || symbols[0].char == '\t') {
		symbols = symbols[1:]
	}
	for len(symbols) > 0 && (symbols[len(symbols)-1].char == ' ' || symbols[len(symbols)-1].char == '\t') {
		symbols = symbols[:len(symbols)-1]
	}
	return symbols
}

// v197Table сохраняет обычные/разделённые ячейки, пустые ячейки и части.
func v197Table(symbols []v197Symbol) (string, int, bool) {
	parts := []string{"row"}
	first, split := 0, false
	for i := 0; i <= len(symbols); i++ {
		if i < len(symbols) {
			symbol := symbols[i]
			if !symbol.escaped && symbol.char == '<' && len(v197TrimSymbols(symbols[first:i])) == 0 {
				split = true
			}
			if !symbol.escaped && symbol.char == '>' && split {
				split = false
			}
			if symbol.char != '|' || symbol.escaped || split {
				continue
			}
		}
		cell := v197TrimSymbols(symbols[first:i])
		if len(cell) > 0 && cell[0].char == '<' && !cell[0].escaped {
			if len(cell) < 2 || cell[len(cell)-1].char != '>' || cell[len(cell)-1].escaped {
				return "", 0, false
			}
			inner := cell[1 : len(cell)-1]
			pieces := []string{"split"}
			from := 0
			for j, symbol := range inner {
				if !symbol.escaped && (symbol.char == '<' || symbol.char == '>') {
					return "", 0, false
				}
				if symbol.char == '|' && !symbol.escaped {
					pieces = append(pieces, v197SymbolText(v197TrimSymbols(inner[from:j])))
					from = j + 1
				}
			}
			pieces = append(pieces, v197SymbolText(v197TrimSymbols(inner[from:])))
			if len(pieces) < 3 {
				return "", 0, false
			}
			parts = append(parts, v197Key(pieces...))
		} else {
			parts = append(parts, v197Key("cell", v197SymbolText(cell)))
		}
		first = i + 1
	}
	if split {
		return "", 0, false
	}
	return v197Key(parts...), len(parts) - 1, true
}

func v197Media(line diagnostics.Line) (v197Unit, bool) {
	if line.Type() != "tag" {
		return v197Unit{}, false
	}
	start, end, valid := v197Argument(line)
	if !valid {
		return v197Unit{}, false
	}
	raw := []rune(line.Raw())
	middle := start
	for middle < end && raw[middle] != ' ' {
		middle++
	}
	kind := strings.ToLower(string(raw[start:middle]))
	if kind != "audio" && kind != "video" && kind != "image" {
		return v197Unit{}, false
	}
	for middle < end && raw[middle] == ' ' {
		middle++
	}
	if middle == end {
		return v197Unit{}, false
	}
	source := string(raw[middle:end])
	sourceKind := "path"
	if raw[middle] == '"' {
		symbols := v197Symbols(source, "source", false)
		if len(symbols) < 2 || symbols[len(symbols)-1].char != '"' || symbols[len(symbols)-1].escaped {
			return v197Unit{}, false
		}
		for _, symbol := range symbols[1 : len(symbols)-1] {
			if symbol.char == '"' && !symbol.escaped {
				return v197Unit{}, false
			}
		}
		source = v197SymbolText(symbols[1 : len(symbols)-1])
	} else {
		if strings.ContainsAny(source, " \t\"") {
			return v197Unit{}, false
		}
		if strings.Trim(source, "0123456789") == "" {
			sourceKind = "number"
		}
	}
	// Готовый SOURCE сохраняет семантику, а исходные кавычки определяют его вид.
	for i := 0; i < line.ElementCount(); i++ {
		element, _ := line.Element(i)
		if element.Type() == "source" && element.Start() == middle+1 && element.End() == end+1 {
			if value, present := element.Value(); present {
				source = value
			}
		}
	}
	if strings.Trim(source, " \t") == "" {
		return v197Unit{}, false
	}
	return v197Unit{v197Key("media", kind, sourceKind, source), v197Range{line.Number(), 1, line.Number(), len(raw) + 1}}, true
}
