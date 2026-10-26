package checkers

import (
	"strings"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV169 регистрирует частичное заполнение ответов matching для DSL 1.2.
func registrationV169(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code: "V169", Severity: diagnostics.SeverityError, Scope: diagnostics.ScopeBlock,
		Message:  "Ответы заданы только для части элементов верхнего списка.",
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV169,
	})
}

// checkV169 самостоятельно устанавливает соседство, не читая другие диагностики.
// Основной диапазон охватывает блок от имени тега до конца закрывающей скобки;
// связанные диапазоны — целые исходные строки. Unicode-колонки с 1, конец исключается.
func checkV169(lines diagnostics.Lines, findings diagnostics.Findings) error {
	bounds := v169Boundaries(lines)
	for number := 1; number <= lines.Len(); number++ {
		block, _ := lines.Line(number)
		name, tag := v169Tag(block)
		if block.Type() != "block-start" || name != "matching" {
			continue
		}
		end := v169BlockEnd(lines, number, bounds[number])
		if end == 0 {
			continue
		}
		separator, separators := 0, 0
		for current := number + 1; current < end; current++ {
			line, _ := lines.Line(current)
			if v169Direct(line, block) && strings.Trim(line.Raw(), " \t") == "---" {
				separator, separators = current, separators+1
			}
		}
		if separators != 1 {
			continue
		}
		missing, answered, eligible := v169Upper(lines, block, separator)
		if !eligible || answered == 0 || len(missing) == 0 {
			continue
		}
		closing, _ := lines.Line(end)
		brace := v169Element(closing, "block-close")
		occurrence, err := findings.Add(number, tag.Start(), end, brace.End(), "")
		if err != nil {
			return err
		}
		for _, missingNumber := range missing {
			line, _ := lines.Line(missingNumber)
			if err := findings.AddRelated(occurrence, missingNumber, 1, missingNumber, utf8.RuneCountInString(line.Raw())+1); err != nil {
				return err
			}
		}
	}
	return nil
}

// v169Upper возвращает строки без ответа и число элементов с ответом.
// eligible=false означает, что хотя бы один присутствующий answer не следует
// непосредственно за элементом этого списка. pending хранит только последний
// элемент до следующей значимой конструкции; далёкие ответы не ищутся.
func v169Upper(lines diagnostics.Lines, block diagnostics.Line, separator int) (missing []int, answered int, eligible bool) {
	pending := 0
	for number := block.Number() + 1; number < separator; number++ {
		line, _ := lines.Line(number)
		direct := v169Direct(line, block)
		name, _ := v169Tag(line)
		if name == "answer" {
			if !direct || pending == 0 {
				return nil, 0, false
			}
			answered++
			missing = missing[:len(missing)-1]
			pending = 0
			continue
		}
		if direct && strings.Trim(line.Raw(), " \t") == "" {
			continue
		}
		pending = 0
		if direct && line.Type() == "content" && strings.Trim(line.Raw(), " \t") != "" {
			missing = append(missing, number)
			pending = number
		}
	}
	return missing, answered, true
}

// v169Direct проверяет непосредственного родителя и глубину готовой модели.
func v169Direct(line, block diagnostics.Line) bool {
	parent, found := line.ParentLine()
	return found && parent == block.Number() && line.NestingLevel() == block.NestingLevel()+1
}

// v169Element возвращает единственный элемент заданного типа либо nil.
func v169Element(line diagnostics.Line, kind string) diagnostics.Element {
	var found diagnostics.Element
	for i := 0; i < line.ElementCount(); i++ {
		element, _ := line.Element(i)
		if element.Type() == kind {
			if found != nil {
				return nil
			}
			found = element
		}
	}
	return found
}

// v169Tag читает начальное распознанное объявление; литеральный текст не разбирается.
// Ошибка в остатке invalid-строки не отменяет уже распознанный tag.
func v169Tag(line diagnostics.Line) (string, diagnostics.Element) {
	if line.Type() != "tag" && line.Type() != "block-start" && line.Type() != "invalid" {
		return "", nil
	}
	tag := v169Element(line, "tag")
	start := len(line.Raw()) - len(strings.TrimLeft(line.Raw(), " \t"))
	if tag == nil || tag.Start() != start+1 || !strings.HasPrefix(line.Raw()[start:], "@") {
		return "", nil
	}
	name, _ := tag.Value()
	return strings.ToLower(name), tag
}

// v169Boundary хранит включающую правую границу и размер поддерева строки.
// end=-1 означает повторное закрытие; nestedEnd — самое позднее закрытие
// вложенного block-start, Len()+1 при отсутствующем или повторном закрытии.
type v169Boundary struct {
	end, last, size, nestedEnd int
}

// v169Boundaries за два линейных прохода индексирует все родительские поддеревья.
// Prepare гарантирует, что родитель предшествует ребёнку: обратный проход
// передаёт ему готовые итоги, не обходя цепочку предков для каждой строки.
func v169Boundaries(lines diagnostics.Lines) []v169Boundary {
	bounds := make([]v169Boundary, lines.Len()+1)
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		bounds[number].last, bounds[number].size = number, 1
		if parent, found := line.ParentLine(); found && line.Type() == "block-end" {
			if bounds[parent].end != 0 {
				bounds[parent].end = -1
			} else {
				bounds[parent].end = number
			}
		}
	}
	for number := lines.Len(); number > 0; number-- {
		line, _ := lines.Line(number)
		parent, found := line.ParentLine()
		if !found {
			continue
		}
		current := bounds[number]
		nestedEnd := current.nestedEnd
		if line.Type() == "block-start" {
			if current.end <= number {
				nestedEnd = lines.Len() + 1
			} else {
				nestedEnd = max(nestedEnd, current.end)
			}
		}
		bounds[parent].last = max(bounds[parent].last, current.last)
		bounds[parent].size += current.size
		bounds[parent].nestedEnd = max(bounds[parent].nestedEnd, nestedEnd)
	}
	return bounds
}

// v169BlockEnd возвращает однозначное структурное закрытие либо ноль.
// Максимум и размер поддерева доказывают непрерывность тела: все потомки
// следуют за opening, поэтому size=end-opening+1 и last=end исключают
// как посторонние строки внутри, так и потомков после закрытия.
func v169BlockEnd(lines diagnostics.Lines, opening int, bounds v169Boundary) int {
	end := bounds.end
	if end <= opening || bounds.last != end || bounds.size != end-opening+1 || bounds.nestedEnd >= end {
		return 0
	}
	first, _ := lines.Line(opening)
	last, _ := lines.Line(end)
	left, right := v169Element(first, "block-open"), v169Element(last, "block-close")
	if left == nil || right == nil || strings.Trim(last.Raw(), " \t") != "}" ||
		strings.Trim(string([]rune(first.Raw())[left.End()-1:]), " \t") != "" {
		return 0
	}
	return end
}
