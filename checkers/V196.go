package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// v196Block хранит физические границы из модели и диапазон тега контейнера.
// close=0 означает неопределённое закрытие; колонки имеют исключающий конец.
type v196Block struct {
	open, close, start, end int
	name                    string
	uncertain               bool
}

// registrationV196 регистрирует запрет newpage в variants только для DSL 1.2.
func registrationV196(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code: "V196", Severity: diagnostics.SeverityError,
		Message:  "@newpage размещён внутри @variants.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV196,
	})
}

// checkV196 не использует логическую границу newpage для закрытия контейнера.
// Исключения определяются ближайшим фигурным блоком без чужих результатов.
func checkV196(lines diagnostics.Lines, findings diagnostics.Findings) error {
	blocks := v196Blocks(lines)
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		start, end := v196Newpage(line)
		if start == 0 {
			continue
		}
		container, known := v196Container(blocks, number)
		if !known || container.name != "variants" {
			continue
		}
		occurrence, err := findings.Add(number, start, number, end, "")
		if err != nil {
			return err
		}
		if err := findings.AddRelated(occurrence, container.open, container.start, container.open, container.end); err != nil {
			return err
		}
	}
	return nil
}

// v196Blocks получает интервалы из block-start/block-end и ParentLine закрытия.
// Текст скобок повторно не разбирается. Неоднозначные границы сохраняются как
// неизвестные, чтобы они не открывали доступ к более дальнему variants.
func v196Blocks(lines diagnostics.Lines) []v196Block {
	ends := make(map[int]int)
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if line.Type() != "block-end" {
			continue
		}
		parent, _ := line.ParentLine()
		if _, exists := ends[parent]; exists {
			ends[parent] = -1
		} else {
			ends[parent] = number
		}
	}
	var blocks []v196Block
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if line.Type() != "block-start" {
			continue
		}
		block := v196Block{open: number}
		if tag := v196Element(line, "tag"); tag != nil {
			value, _ := tag.Value()
			block.name = strings.ToLower(value)
			block.start, block.end = tag.Start(), tag.End()
		}
		if end := ends[number]; end > number && v196Element(line, "block-open") != nil {
			closing, _ := lines.Line(end)
			if v196Element(closing, "block-close") != nil {
				block.close = end
			}
		}
		blocks = append(blocks, block)
	}
	// Связанные закрытия существуют, но пересекающиеся интервалы не могут
	// задавать фигурную вложенность. Не используем ни один из них как владельца.
	for left := range blocks {
		for right := left + 1; right < len(blocks); right++ {
			if blocks[left].close != 0 && blocks[right].open >= blocks[left].close {
				break
			}
			if blocks[left].close != 0 && blocks[right].close > blocks[left].close {
				blocks[left].uncertain, blocks[right].uncertain = true, true
			}
		}
	}
	return blocks
}

// v196Container возвращает ближайший физически содержащий блок.
// known=false означает отсутствие блока, литеральный контекст или неоднозначные
// ближайшие границы. Ограниченный внутренний блок восстанавливает определимость
// после незакрытых или пересечённых внешних блоков. Литеральный предок запрещает
// такое восстановление. V124 использует тот же контекст для newpage.
func v196Container(blocks []v196Block, number int) (nearest v196Block, known bool) {
	for _, block := range blocks {
		if block.open >= number {
			break
		}
		if block.close != 0 && block.close <= number {
			continue
		}
		if block.name == "editor" || block.name == "multifill" {
			return v196Block{}, false
		}
		if block.uncertain || block.close == 0 {
			nearest, known = v196Block{}, false
			continue
		}
		if known && block.close >= nearest.close {
			return v196Block{}, false
		}
		nearest, known = block, true
	}
	return nearest, known
}

// v196Newpage возвращает диапазон начального объявления либо (0,0).
// При отсутствии tag имя в Raw завершается только концом строки, U+0020,
// U+0009 или {. Отступ ASCII не декодируется; диапазон исключает остаток строки.
func v196Newpage(line diagnostics.Line) (int, int) {
	raw := line.Raw()
	start := 0
	for start < len(raw) && (raw[start] == ' ' || raw[start] == '\t') {
		start++
	}
	if start == len(raw) || raw[start] != '@' {
		return 0, 0
	}
	hasTag := false
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() != "tag" {
			continue
		}
		hasTag = true
		value, _ := element.Value()
		if element.Start() == start+1 && strings.EqualFold(value, "newpage") {
			return element.Start(), element.End()
		}
	}
	if hasTag || len(raw)-start < len("@newpage") {
		return 0, 0
	}
	end := start + len("@newpage")
	if !strings.EqualFold(raw[start:end], "@newpage") {
		return 0, 0
	}
	if end < len(raw) && raw[end] != ' ' && raw[end] != '\t' && raw[end] != '{' {
		return 0, 0
	}
	// До конца имени находятся только ASCII-отступ и ASCII-имя.
	return start + 1, end + 1
}

// v196Element возвращает единственный элемент типа либо nil при отсутствии
// или неоднозначности. Позиции элемента относятся к исходному документу.
func v196Element(line diagnostics.Line, kind string) diagnostics.Element {
	var found diagnostics.Element
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == kind {
			if found != nil {
				return nil
			}
			found = element
		}
	}
	return found
}
