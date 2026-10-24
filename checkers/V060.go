package checkers

import (
	"fmt"
	"math/big"
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV060 регистрирует проверку числовой последовательности шагов DSL 1.2.
func registrationV060(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V060",
		Severity: diagnostics.SeverityRecommendation,
		Message:  "Нарушена числовая последовательность заголовков шагов.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV060,
	})
}

// v060Number связывает значение произвольной длины с исходной группой цифр.
// Колонки Unicode начинаются с 1; end — исключающая правая граница.
type v060Number struct {
	value            *big.Int
	line, start, end int
}

// checkV060 проверяет числа по порядку исходника, без сортировки и удаления
// повторов. После нарушения отсчёт продолжается от фактического текущего числа.
func checkV060(lines diagnostics.Lines, findings diagnostics.Findings) error {
	var previous *v060Number
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		if !v060StepHeading(line) {
			continue
		}
		for index := 0; index < line.ElementCount(); index++ {
			title, _ := line.Element(index)
			if title.Type() != "title" {
				continue
			}
			numbers, err := v060TitleNumbers(number, title)
			if err != nil {
				return err
			}
			for _, current := range numbers {
				detail := v060Detail(current.value, previous)
				if detail != "" {
					occurrence, err := findings.Add(current.line, current.start, current.line, current.end, detail)
					if err != nil {
						return err
					}
					if previous != nil {
						if err := findings.AddRelated(occurrence, previous.line, previous.start, previous.line, previous.end); err != nil {
							return err
						}
					}
				}
				previous = &current
			}
		}
	}
	return nil
}

// v060StepHeading выбирает распознанный корневой заголовок уровня 3.
// Литеральные строки и вложенные попытки не являются такими заголовками
// по контракту модели; распознавание псевдозаголовков из Raw не выполняется.
func v060StepHeading(line diagnostics.Line) bool {
	if line.Type() != "heading" {
		return false
	}
	if _, found := line.ParentLine(); found {
		return false
	}
	for index := 0; index < line.ElementCount(); index++ {
		element, _ := line.Element(index)
		if element.Type() == "heading-level" && element.Raw() == "###" {
			return true
		}
	}
	return false
}

// v060TitleNumbers извлекает все максимальные группы Nd из исходного title.
// Нормализуется только числовое значение; группы, разделённые любым не-Nd,
// не склеиваются. Исходные диапазоны сохраняют письменность и ведущие нули.
func v060TitleNumbers(line int, title diagnostics.Element) ([]v060Number, error) {
	raw := []rune(title.Raw())
	var numbers []v060Number
	for index := 0; index < len(raw); {
		if v060Digit(raw[index]) < 0 {
			index++
			continue
		}
		start := index
		var decimal []byte
		for index < len(raw) {
			digit := v060Digit(raw[index])
			if digit < 0 {
				break
			}
			decimal = append(decimal, byte('0'+digit))
			index++
		}
		value, ok := new(big.Int).SetString(string(decimal), 10)
		if !ok {
			return nil, fmt.Errorf("не удалось получить десятичное значение группы Nd в строке %d", line)
		}
		numbers = append(numbers, v060Number{value: value, line: line,
			start: title.Start() + start, end: title.Start() + index})
	}
	return numbers, nil
}

// v060Digit возвращает десятичное значение символа Nd или -1 для другого символа.
// В таблицах Nd наборы идут от 0 до 9; диапазон может объединять несколько
// наборов (в частности, математические цифры), поэтому значение берётся по mod 10.
func v060Digit(char rune) int {
	code := uint32(char)
	if code <= 0xffff {
		for _, span := range unicode.Nd.R16 {
			if code < uint32(span.Lo) {
				break
			}
			if code <= uint32(span.Hi) && (code-uint32(span.Lo))%uint32(span.Stride) == 0 {
				return int((code - uint32(span.Lo)) / uint32(span.Stride) % 10)
			}
		}
	} else {
		for _, span := range unicode.Nd.R32 {
			if code < span.Lo {
				break
			}
			if code <= span.Hi && (code-span.Lo)%span.Stride == 0 {
				return int((code - span.Lo) / span.Stride % 10)
			}
		}
	}
	return -1
}

// v060Detail описывает нарушение начала, повтор, уменьшение или пропуск.
// Пустая строка означает правильное начало либо увеличение ровно на единицу.
// Переданные значения не изменяются.
func v060Detail(current *big.Int, previous *v060Number) string {
	one := big.NewInt(1)
	if previous == nil {
		if current.Cmp(one) == 0 {
			return ""
		}
		detail := fmt.Sprintf("Ожидалось начало с 1; найдено %s.", current)
		if current.Cmp(one) > 0 {
			detail += " " + v060Gap(one, new(big.Int).Sub(current, one))
		}
		return detail
	}
	switch current.Cmp(previous.value) {
	case 0:
		return fmt.Sprintf("Повтор значения: %s.", current)
	case -1:
		return fmt.Sprintf("Нарушен порядок: после %s следует %s.", previous.value, current)
	}
	next := new(big.Int).Add(previous.value, one)
	if current.Cmp(next) > 0 {
		return v060Gap(next, new(big.Int).Sub(current, one))
	}
	return ""
}

// v060Gap форматирует один пропуск или включительный диапазон без перечисления.
func v060Gap(first, last *big.Int) string {
	if first.Cmp(last) == 0 {
		return fmt.Sprintf("Пропущено значение: %s.", first)
	}
	return fmt.Sprintf("Пропущены значения: %s … %s.", first, last)
}
