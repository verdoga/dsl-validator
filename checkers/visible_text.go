package checkers

import (
	"strings"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// textFormatting отмечает разрешённое форматирование внутри content блока
// text. Распознавание не требует запуска других диагностик.
// Отдельный <br> остаётся границей; литеральные и экранированные теги не скрыты.
func textFormatting(lines diagnostics.Lines, line diagnostics.Line, raw []rune) []bool {
	if !v016InsideText(lines, line) || strings.EqualFold(strings.Trim(line.Raw(), " \t"), "<br>") {
		return nil
	}
	return v016HTMLMarkers(line, raw)
}

// visibleText исключает только отмеченные маркеры из [start,end). positions
// хранит исходный индекс каждой оставшейся кодовой точки; Raw не изменяется.
func visibleText(raw []rune, formatting []bool, start, end int) (text []rune, positions []int) {
	text = make([]rune, 0, end-start)
	positions = make([]int, 0, end-start)
	for i := start; i < end; i++ {
		if len(formatting) != 0 && formatting[i] {
			continue
		}
		text = append(text, raw[i])
		positions = append(positions, i)
	}
	return text, positions
}
