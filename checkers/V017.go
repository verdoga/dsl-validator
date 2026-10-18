package checkers

import (
	"unicode"

	"github.com/verdoga/dsl-validator/diagnostics"
)

// registrationV017 регистрирует предупреждение о пробелах перед пунктуацией
// только для DSL 1.2.
func registrationV017(registry *diagnostics.Registry) error {
	return registry.Register(diagnostics.Definition{
		Code:     "V017",
		Severity: diagnostics.SeverityWarning,
		Message:  "Перед знаком препинания находится пробел.",
		Scope:    diagnostics.ScopeLine,
		Versions: diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"},
		Check:    checkV017,
	})
}

// checkV017 проверяет неизменённый Raw элементов content/title/name.
// HTML не распознаётся; соседство ограничено одним элементом одной строки.
// Отмечаются только пробелы в исходных Unicode-колонках с 1, конец исключается.
func checkV017(lines diagnostics.Lines, findings diagnostics.Findings) error {
	for number := 1; number <= lines.Len(); number++ {
		line, _ := lines.Line(number)
		for index := 0; index < line.ElementCount(); index++ {
			element, _ := line.Element(index)
			switch element.Type() {
			case "content", "title", "name":
			default:
				continue
			}
			raw := []rune(element.Raw())
			for i := 0; i < len(raw); {
				switch raw[i] {
				case ',', '.', '?', '!', '…':
				default:
					i++
					continue
				}
				start := i
				for start > 0 && v017HorizontalSpace(raw[start-1]) {
					start--
				}
				if start < i {
					if _, err := findings.Add(number, element.Start()+start, number, element.Start()+i, ""); err != nil {
						return err
					}
				}
				if raw[i] == '.' {
					end, count, spaced := v017DotChain(raw, i)
					if count >= 3 && spaced {
						// Пробелы перед первой точкой уже проверены, внутренние
						// пробелы всей максимальной цепочки исключаются.
						i = end
						continue
					}
					if !spaced {
						// Цельное ... даёт одно срабатывание. В более длинной
						// непрерывной серии также нет внутренних пробелов.
						i = end
						continue
					}
				}
				i++
			}
		}
	}
	return nil
}

// v017DotChain находит максимальную цепочку точек и горизонтальных промежутков
// от start, указывающего на точку. end — исключающий индекс после последней
// точки в кодовых точках с 0; count — число точек. spaced=true означает хотя бы
// один непустой внутренний промежуток; конечные пробелы в цепочку не входят.
func v017DotChain(raw []rune, start int) (end, count int, spaced bool) {
	end, count = start+1, 1
	for {
		next := end
		for next < len(raw) && v017HorizontalSpace(raw[next]) {
			next++
		}
		if next == len(raw) || raw[next] != '.' {
			return end, count, spaced
		}
		spaced = spaced || next > end
		end, count = next+1, count+1
	}
}

// v017HorizontalSpace возвращает true для U+0009 и Unicode Zs;
// false означает символ, не входящий в проверяемую последовательность пробелов.
func v017HorizontalSpace(char rune) bool {
	return char == '\t' || unicode.Is(unicode.Zs, char)
}
