package checkers

import "unicode"

// v021EndsColon проверяет конечное двоеточие видимого текста, пропуская
// закрывающие Pe/Pf и ASCII-кавычки так же, как v021Unfinished.
func v021EndsColon(text []rune) bool {
	end := len(text)
	for end > 0 && (unicode.In(text[end-1], unicode.Pe, unicode.Pf) ||
		text[end-1] == '\'' || text[end-1] == '"') {
		end--
	}
	return end > 0 && text[end-1] == ':'
}

// v021MarkInstrRows отмечает минимум два пункта непосредственно после
// вводного двоеточия. Вызывается только для непосредственного содержимого instr.
// Пустые строки и конструкции не пропускаются: нужны физические соседи.
// Пункты — фразы; конечная точка допустима и завершает последовательность.
// Дальнейший текст снова проверяется обычным образом.
func v021MarkInstrRows(rows []v021Row, independent []bool) {
	for i := 0; i < len(rows); i++ {
		if !v021EndsColon(rows[i].text) || !v021HasLetter(rows[i].text) {
			continue
		}
		end := i + 1
		for end < len(rows) && rows[end].number == rows[end-1].number+1 {
			text := rows[end].text
			terminal := len(text) > 0 && text[len(text)-1] == '.'
			if terminal {
				text = text[:len(text)-1]
			}
			if !v021Phrase(text) {
				break
			}
			end++
			if terminal {
				break
			}
		}
		if end-i < 3 {
			continue
		}
		for _, row := range rows[i+1 : end] {
			independent[row.number] = true
		}
		i = end - 1
	}
}
