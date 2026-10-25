package checkers

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV075 регистрирует проверку каждого явно объявленного этапа DSL 1.2.
func registrationV075(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code: "V075", Severity: diagnostics.SeverityError, Scope: diagnostics.ScopeBlock,
		Message:  "Внутренний этап после @step не содержит учебного материала.",
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV075,
	})
}

// v075Material различает доказанную пустоту, неопределённость и найденный материал.
// Наличие материала достаточно независимо от ошибок в остальных частях участка.
type v075Material int

const (
	v075Empty v075Material = iota
	v075Unknown
	v075Filled
)

type v075Fragment struct {
	id   string
	line int
}

// v075FragmentState хранит результат и состояние обхода графа вставок.
// pending означает, что результат ещё зависит от незавершённого цикла.
// index/low — номер посещения и минимальный достижимый номер в стеке Тарьяна.
type v075FragmentState struct {
	index, low int
	material   v075Material
	pending    bool
}

// v075Scan хранит только данные текущего вызова; границы блоков берутся из модели.
type v075Scan struct {
	lines     diagnostics.Lines
	ends      map[int]int
	fragments []v075Fragment
	states    map[int]*v075FragmentState
	stack     []int
}

// checkV075 завершает последний этап на каждой границе задания и на EOF.
func checkV075(lines diagnostics.Lines, findings diagnostics.Findings) error {
	scan := v075Index(lines)
	active, stream, step, column := false, 0, 0, 0
	finish := func(last, boundary int) error {
		if step == 0 || scan.material(step+1, last, 0) != v075Empty {
			return nil
		}
		line, _ := lines.Line(last)
		occurrence, err := findings.Add(step, column, last, utf8.RuneCountInString(line.Raw())+1, "")
		if err != nil {
			return err
		}
		if boundary != 0 {
			line, _ = lines.Line(boundary)
			return findings.AddRelated(occurrence, boundary, 1, boundary, utf8.RuneCountInString(line.Raw())+1)
		}
		return nil
	}
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		context := v075Context(lines, line)
		// Вложенные объявления не меняют структурные границы задания.
		// Их содержимое исследуется отдельно при завершении настоящего этапа.
		if context.content {
			continue
		}
		if stream != context.variants {
			// Если парсер не сохранил закрытие ветви, её конец неопределён.
			// Штатные границы обрабатываются явно до смены потока.
			active, step, stream = false, 0, context.variants
		}
		name, start, _ := v075Tag(line)
		boundary := false
		switch name {
		case "task", "endtask", "header", "newpage", "variants":
			boundary = true
		case "variant":
			boundary = context.variants != 0
		}
		if level := v075Element(line, "heading-level"); level != nil {
			value, _ := level.Value()
			boundary = boundary || value == "3"
		}
		if line.Type() == "block-end" {
			parent, found := line.ParentLine()
			boundary = boundary || found && parent == context.variants
		}
		if boundary || name == "step" && active {
			if err := finish(number-1, number); err != nil {
				return err
			}
			step = 0
			if boundary {
				active = name == "task"
			} else {
				step, column = number, start
			}
		}
	}
	return finish(lines.Len(), 0)
}

// v075Index связывает объявления блоков с закрытиями и сохраняет все определения
// fragment, включая повторные ID. Неоднозначное закрытие имеет значение -1.
func v075Index(lines diagnostics.Lines) v075Scan {
	scan := v075Scan{lines: lines, ends: make(map[int]int), states: make(map[int]*v075FragmentState)}
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if line.Type() == "block-end" {
			if parent, found := line.ParentLine(); found {
				if _, exists := scan.ends[parent]; exists {
					scan.ends[parent] = -1
				} else {
					scan.ends[parent] = number
				}
			}
		}
		name, _, end := v075Tag(line)
		context := v075Context(lines, line)
		if name == "fragment" && line.Type() == "block-start" && !context.editor && !context.multifill {
			if id, known := v075ID(line, end); known {
				scan.fragments = append(scan.fragments, v075Fragment{id, number})
			}
		}
	}
	return scan
}

// material исследует участок исходника либо тело указанного fragment.
// Кеш вставок общий для всех участков документа и не хранит позиции срабатываний.
func (scan *v075Scan) material(first, last, fragment int) v075Material {
	state := v075Empty
	for number := first; number <= last; number++ {
		line, _ := scan.lines.Line(number)
		context := v075Context(scan.lines, line)
		if context.editor || context.fragment != fragment {
			continue
		}
		name, _, end := v075Tag(line)
		if !context.multifill && (name == "editor" || name == "fragment" || name == "speaking") {
			continue
		}
		if line.Type() == "blank" || line.Type() == "separator" || !v075Nonblank(line.Raw()) {
			continue
		}
		current := v075Empty
		if name == "include" && !context.multifill {
			current = scan.include(line, end, fragment)
		} else {
			current = v075LineMaterial(line, name, end, context.multifill)
		}
		if current == v075Filled {
			return v075Filled
		}
		if line.Type() == "block-start" && (scan.ends[number] <= number || scan.ends[number] > last) {
			current = v075Unknown
		}
		if current == v075Unknown {
			state = v075Unknown
		}
	}
	return state
}

// include исследует только единственное определение с совпадающим ID.
// Проверки самой ссылки предшествуют кешу: ошибочный хвост или неоднозначный ID
// не должны получать результат ранее исследованного корректного включения.
func (scan *v075Scan) include(line diagnostics.Line, end, owner int) v075Material {
	id, known := v075ID(line, end)
	if !known || v075ReferenceExtra(line, end) {
		return v075Unknown
	}
	opening := 0
	for _, fragment := range scan.fragments {
		if strings.EqualFold(id, fragment.id) {
			if opening != 0 {
				return v075Unknown
			}
			opening = fragment.line
		}
	}
	if opening == 0 || scan.ends[opening] <= opening {
		return v075Unknown
	}
	definition, _ := scan.lines.Line(opening)
	_, _, definitionEnd := v075Tag(definition)
	if v075ReferenceExtra(definition, definitionEnd) {
		return v075Unknown
	}
	return scan.fragmentMaterial(opening, owner)
}

// fragmentMaterial вычисляет каждый достижимый фрагмент не более одного раза.
// Незавершённая обратная ссылка даёт unknown, но не становится окончательным
// результатом. Компонента взаимных ссылок завершается целиком: материал любого
// её участника достижим из остальных. Без материала цикл остаётся unknown.
// Уже завершённые компоненты переиспользуются в любом участке пути и этапа.
func (scan *v075Scan) fragmentMaterial(opening, owner int) v075Material {
	if state, found := scan.states[opening]; found {
		if state.pending && owner != 0 {
			parent := scan.states[owner]
			parent.low = min(parent.low, state.index)
		}
		return state.material
	}
	index := len(scan.states)
	state := &v075FragmentState{index: index, low: index, material: v075Unknown, pending: true}
	scan.states[opening] = state
	scan.stack = append(scan.stack, opening)
	// У штатного закрытия нет материала. Ошибочный остаток его строки
	// также исследуется, чтобы не объявить такой фрагмент заведомо пустым.
	state.material = scan.material(opening+1, scan.ends[opening], opening)
	if state.low == state.index {
		start := len(scan.stack) - 1
		material := state.material
		for scan.stack[start] != opening {
			material = max(material, scan.states[scan.stack[start]].material)
			start--
		}
		for _, member := range scan.stack[start:] {
			cached := scan.states[member]
			cached.material, cached.pending = material, false
		}
		scan.stack = scan.stack[:start]
	} else if owner != 0 {
		parent := scan.states[owner]
		parent.low = min(parent.low, state.low)
	}
	return state.material
}

// v075ReferenceExtra обнаруживает лишний текст вокруг уже распознанного ID.
// При восстановленном ID весь остаток перед скобкой проверяет v075ID.
func v075ReferenceExtra(line diagnostics.Line, end int) bool {
	raw := []rune(line.Raw())
	limit := len(raw)
	if brace := v075Element(line, "block-open"); brace != nil {
		if v075Nonblank(string(raw[brace.End()-1:])) {
			return true
		}
		limit = brace.Start() - 1
	}
	if id := v075Element(line, "identifier"); id != nil {
		return end == 0 || id.Start() < end || id.End()-1 > limit ||
			v075Nonblank(string(raw[end-1:id.Start()-1])) ||
			v075Nonblank(string(raw[id.End()-1:limit]))
	}
	return false
}

// v075LineMaterial использует значения content, а затем исследует доступный Raw.
// Названия и служебные поля не превращаются в текст при восстановлении остатка.
func v075LineMaterial(line diagnostics.Line, name string, end int, literal bool) v075Material {
	if line.Type() == "heading" || v075Element(line, "heading-level") != nil {
		return v075Empty
	}
	if name == "media" && !literal {
		kind, source := v075Element(line, "media-type"), v075Element(line, "source")
		if kind != nil && source != nil {
			k, _ := kind.Value()
			s, _ := source.Value()
			if v075Nonblank(k) && v075Nonblank(s) {
				return v075Filled
			}
		}
		return v075Media(string([]rune(line.Raw())[end-1:]))
	}
	textual := literal || name == ""
	switch name {
	case "instr", "note", "hint", "example", "wordlist", "table", "script", "text", "key", "alt",
		"question", "multifill", "choice", "multichoice", "matching", "ordering", "answer":
		textual = true
	case "task", "endtask", "header", "newpage", "variants", "variant", "step", "speaking",
		"dsl-version", "document-id", "section", "order", "resource-dir":
		if !literal {
			return v075Empty
		}
	}
	if !textual {
		return v075Unknown
	}
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "content" {
			value, present := element.Value()
			if v075Nonblank(value) || !present && v075Nonblank(element.Raw()) {
				return v075Filled
			}
		}
	}
	// У блочных title нет роли материала, даже если парсер сохранил их
	// в unparsed. Инструкция интерактивного блока, напротив, является content.
	if line.Type() == "block-start" {
		switch name {
		case "example", "wordlist", "table", "script", "text", "key", "note", "alt":
			// Название находится до {. Неразобранный текст после скобки
			// не относится к названию и должен остаться для осмотра.
			end = v075Element(line, "block-open").End()
		}
	}
	raw := []rune(line.Raw())
	// Удаление здесь означает лишь маскирование уже классифицированных
	// диапазонов при осмотре остатка, без изменения документа или Value.
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() != "unparsed" {
			for i := element.Start() - 1; i < element.End()-1; i++ {
				raw[i] = ' '
			}
		}
	}
	if name != "" && !literal {
		for i := 0; i < end-1; i++ {
			raw[i] = ' '
		}
	}
	if !v075Nonblank(string(raw)) {
		return v075Empty
	}
	remaining := strings.TrimLeftFunc(string(raw), unicode.IsSpace)
	if !literal && (strings.ContainsAny(remaining, "{}") || name == "" && strings.ContainsAny(remaining[:1], "@#")) {
		return v075Unknown
	}
	return v075Filled
}

// v075Media проверяет наличие двух исходных полей без разрешения ресурса
// и без декодирования пути. Неоднозначные кавычки/лишние поля дают unknown.
func v075Media(tail string) v075Material {
	tail = strings.TrimSpace(tail)
	if tail == "" {
		return v075Empty
	}
	split := strings.IndexFunc(tail, unicode.IsSpace)
	if split < 0 {
		return v075Empty
	}
	source := strings.TrimSpace(tail[split:])
	if source == "" {
		return v075Empty
	}
	if source[0] != '"' {
		if strings.IndexFunc(source, unicode.IsSpace) >= 0 || strings.ContainsAny(source, "\"{}") {
			return v075Unknown
		}
		return v075Filled
	}
	escaped, nonblank := false, false
	for index, char := range source[1:] {
		if escaped {
			if char != '\\' && char != '"' {
				return v075Unknown
			}
			escaped, nonblank = false, true
		} else if char == '\\' {
			escaped = true
		} else if char == '"' {
			if v075Nonblank(source[index+2:]) {
				return v075Unknown
			}
			if nonblank {
				return v075Filled
			}
			return v075Empty
		} else if !unicode.Is(unicode.White_Space, char) {
			nonblank = true
		}
	}
	return v075Unknown
}

// v075ID читает точное семантическое значение либо единственный исходный ID.
// Только служебные крайние U+0020/U+0009 вне исходного ID отделяются при чтении.
func v075ID(line diagnostics.Line, end int) (string, bool) {
	if element := v075Element(line, "identifier"); element != nil {
		value, present := element.Value()
		return value, present && v075Nonblank(value)
	}
	raw := []rune(line.Raw())
	limit := len(raw)
	if brace := v075Element(line, "block-open"); brace != nil {
		limit = brace.Start() - 1
	}
	if end == 0 || end-1 > limit {
		return "", false
	}
	id := strings.Trim(string(raw[end-1:limit]), " \t")
	if id == "" {
		return "", false
	}
	for _, char := range id {
		if !unicode.IsLetter(char) && !unicode.IsDigit(char) && char != '.' && char != '-' && char != '_' {
			return "", false
		}
	}
	return id, true
}

type v075Parents struct {
	variants, fragment         int
	content, editor, multifill bool
}

// v075Context различает структурный поток, определения и литеральные тела.
func v075Context(lines diagnostics.Lines, line diagnostics.Line) v075Parents {
	var context v075Parents
	for parent, found := line.ParentLine(); found; parent, found = line.ParentLine() {
		line, _ = lines.Line(parent)
		if line.Type() != "block-start" {
			continue
		}
		name, _, _ := v075Tag(line)
		if name == "variants" {
			context.variants = parent
		} else {
			context.content, context.variants = true, 0
			switch name {
			case "editor":
				context.editor = true
			case "multifill":
				context.multifill = true
			case "fragment":
				if context.fragment == 0 {
					context.fragment = parent
				}
			}
		}
	}
	return context
}

// v075Tag предпочитает готовый tag и восстанавливает точное имя при unparsed.
// Начальные U+0020/U+0009 пропускаются; позиции исходные, с исключающим концом.
func v075Tag(line diagnostics.Line) (string, int, int) {
	raw := []rune(line.Raw())
	start := 0
	for start < len(raw) && (raw[start] == ' ' || raw[start] == '\t') {
		start++
	}
	if start == len(raw) || raw[start] == '\\' {
		return "", 0, 0
	}
	if element := v075Element(line, "tag"); element != nil {
		value, _ := element.Value()
		return strings.ToLower(value), element.Start(), element.End()
	}
	if raw[start] != '@' || line.Type() != "invalid" && v075Element(line, "unparsed") == nil {
		return "", 0, 0
	}
	end := start + 1
	for end < len(raw) && raw[end] != '{' && raw[end] != '}' && !unicode.Is(unicode.White_Space, raw[end]) {
		end++
	}
	return strings.ToLower(string(raw[start+1 : end])), start + 1, end + 1
}

// v075Element возвращает первый элемент заданного типа либо nil.
func v075Element(line diagnostics.Line, kind string) diagnostics.Element {
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == kind {
			return element
		}
	}
	return nil
}

// v075Nonblank проверяет только Unicode White_Space, не преобразуя строку.
func v075Nonblank(text string) bool {
	for _, char := range text {
		if !unicode.Is(unicode.White_Space, char) {
			return true
		}
	}
	return false
}
