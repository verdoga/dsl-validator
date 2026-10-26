package checkers

import (
	"strings"
	"unicode"
)

// answerPrimary приводит декодированный ответ к нижнему регистру Unicode,
// сворачивает пробельные последовательности и удаляет пробелы по краям.
func answerPrimary(answer string) string {
	return strings.Join(strings.Fields(strings.ToLower(answer)), " ")
}

// answerEqual сравнивает непустые основные формы ответов, а при различии —
// непустые формы без пунктуации Unicode. Пустые ответы всегда дают false.
func answerEqual(left, right string) bool {
	left = answerPrimary(left)
	right = answerPrimary(right)
	if left == "" || right == "" {
		return false
	}
	if left == right {
		return true
	}
	left = answerWithoutPunctuation(left)
	right = answerWithoutPunctuation(right)
	return left != "" && right != "" && left == right
}

// answerPosition возвращает канонический положительный номер из ASCII-цифр
// без ведущих нулей. При неудаче возвращает пустую строку и false.
// Пунктуация Unicode удаляется, только если основная форма не является номером;
// длина номера не ограничена размером машинного целого.
func answerPosition(answer string) (string, bool) {
	isPosition := func(candidate string) bool {
		if candidate == "" || candidate[0] < '1' || candidate[0] > '9' {
			return false
		}
		for i := 1; i < len(candidate); i++ {
			if candidate[i] < '0' || candidate[i] > '9' {
				return false
			}
		}
		return true
	}

	answer = answerPrimary(answer)
	if isPosition(answer) {
		return answer, true
	}
	answer = answerWithoutPunctuation(answer)
	if isPosition(answer) {
		return answer, true
	}
	return "", false
}

// answerWithoutPunctuation удаляет пунктуацию Unicode и повторно сворачивает
// пробельные последовательности, включая пробелы по краям.
func answerWithoutPunctuation(answer string) string {
	answer = strings.Map(func(r rune) rune {
		if unicode.IsPunct(r) {
			return -1
		}
		return r
	}, answer)
	return strings.Join(strings.Fields(answer), " ")
}
