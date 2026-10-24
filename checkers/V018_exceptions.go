package checkers

import "unicode"

// v018AbbreviationDot распознаёт внутреннюю точку отдельного сокращения
// буква.буква. в одном участке. Завершающая точка проверяется обычным образом.
func v018AbbreviationDot(text []rune, index int) bool {
	if index < 1 || index+2 >= len(text) || text[index] != '.' ||
		!unicode.IsLetter(text[index-1]) || !unicode.IsLetter(text[index+1]) || text[index+2] != '.' {
		return false
	}
	if index == 1 {
		return true
	}
	previous := text[index-2]
	return !unicode.IsLetter(previous) && !unicode.IsDigit(previous) &&
		!unicode.IsMark(previous) && previous != '.' && previous != '_'
}

// v018GroupEllipsis разрешает Unicode-многоточие только непосредственно
// после ? или !. Для остальных знаков прежняя проверка пробела сохраняется.
func v018GroupEllipsis(text []rune, index int) bool {
	return index > 0 && text[index] == '…' && (text[index-1] == '?' || text[index-1] == '!')
}
