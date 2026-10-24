package checkers

import "unicode"

// v017V018Detail выбирает пример вокруг [start,end) в видимом тексте одного
// участка. Границы задаются индексами рун с 0. Пример содержит целые токены
// и до трёх токенов контекста вместе с проблемным фрагментом; внутренние
// пробелы сохраняются. Сам проблемный фрагмент никогда не обрезается.
func v017V018Detail(text []rune, start, end int) string {
	for start > 0 && !unicode.IsSpace(text[start-1]) {
		start--
	}
	for end < len(text) && !unicode.IsSpace(text[end]) {
		end++
	}
	count := 0
	inToken := false
	for _, char := range text[start:end] {
		if unicode.IsSpace(char) {
			inToken = false
		} else if !inToken {
			count++
			inToken = true
		}
	}
	// Сначала добавляем правого соседа, затем левого. У края участка
	// недостающий контекст берётся с доступной стороны.
	for count < 3 {
		right := end
		for right < len(text) && unicode.IsSpace(text[right]) {
			right++
		}
		if right < len(text) {
			for right < len(text) && !unicode.IsSpace(text[right]) {
				right++
			}
			end = right
			count++
		}
		if count == 3 {
			break
		}
		left := start
		for left > 0 && unicode.IsSpace(text[left-1]) {
			left--
		}
		if left > 0 {
			for left > 0 && !unicode.IsSpace(text[left-1]) {
				left--
			}
			start = left
			count++
		}
		if right == len(text) && left == 0 {
			break
		}
	}
	for start < end && unicode.IsSpace(text[start]) {
		start++
	}
	for end > start && unicode.IsSpace(text[end-1]) {
		end--
	}
	return "Обратите внимание на знак препинания: «" + string(text[start:end]) + "»"
}
