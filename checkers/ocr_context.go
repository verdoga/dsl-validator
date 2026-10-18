package checkers

import "github.com/verdoga/dsl-validator/diagnostics"

// ocrText возвращает собственную копию Raw для языковых OCR-эвристик.
// Распознанные идентификаторы и источники ресурсов не являются словами или
// числами текста. Их руны заменяются пробелами, сохраняя исходные координаты
// и границы токенов. Остальные участки, включая unparsed, не изменяются.
func ocrText(line diagnostics.Line) []rune {
	raw := []rune(line.Raw())
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		switch element.Type() {
		case "identifier", "resource-path", "source":
			for i := element.Start() - 1; i < element.End()-1; i++ {
				raw[i] = ' '
			}
		}
	}
	return raw
}
