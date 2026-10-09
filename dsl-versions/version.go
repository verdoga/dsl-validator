// Package dslversions задаёт каталог поддерживаемых версий DSL и их числовой
// порядок. Каталог сейчас содержит только 1.2; новые версии добавляются здесь.
// Каталог импорта называется dsl-versions; имя Go-пакета — dslversions.
// Пакет не читает JSON, не меняет модель и не регистрирует диагностики.
package dslversions

import (
	"fmt"
	"strconv"
	"strings"
)

// Version — каноническая версия DSL вида major.minor, отдельная от версии
// JSON-контракта и версии инструмента. Компоненты — десятичные uint64
// без ведущих нулей, кроме самого нуля; пробелы, префиксы и суффиксы запрещены.
type Version string

const (
	// V1_2 — единственная поддерживаемая сейчас версия DSL.
	V1_2 Version = "1.2"
)

// Parse проверяет каноническую запись, не проверяя наличие версии в каталоге.
// Ничего не нормализует; при неверной записи возвращает пустую Version и error.
func Parse(raw string) (Version, error) {
	if _, _, err := parseParts(raw); err != nil {
		return "", err
	}
	return Version(raw), nil
}

// Compare сравнивает числовые компоненты ранее проверенных Parse версий:
// -1 означает left < right, 0 — равенство, 1 — left > right; 1.10 > 1.2.
// Непроверенные и пустые версии передавать нельзя.
func Compare(left, right Version) int {
	leftMajor, leftMinor, _ := parseParts(string(left))
	rightMajor, rightMinor, _ := parseParts(string(right))
	if leftMajor < rightMajor {
		return -1
	}
	if leftMajor > rightMajor {
		return 1
	}
	if leftMinor < rightMinor {
		return -1
	}
	if leftMinor > rightMinor {
		return 1
	}
	return 0
}

// parseParts проверяет каноническую запись и получает обе числовые части;
// неверная форма или переполнение uint64 возвращают error.
func parseParts(raw string) (major, minor uint64, err error) {
	majorRaw, minorRaw, found := strings.Cut(raw, ".")
	if !found || majorRaw == "" || minorRaw == "" {
		return 0, 0, fmt.Errorf("неверная запись версии DSL %q: ожидается major.minor", raw)
	}
	for _, part := range [2]string{majorRaw, minorRaw} {
		if len(part) > 1 && part[0] == '0' {
			return 0, 0, fmt.Errorf("неверная запись версии DSL %q: ведущие нули запрещены", raw)
		}
		for i := 0; i < len(part); i++ {
			if part[i] < '0' || part[i] > '9' {
				return 0, 0, fmt.Errorf("неверная запись версии DSL %q: компоненты должны содержать только десятичные цифры", raw)
			}
		}
	}
	major, err = strconv.ParseUint(majorRaw, 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("неверная старшая компонента версии DSL %q: %w", raw, err)
	}
	minor, err = strconv.ParseUint(minorRaw, 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("неверная младшая компонента версии DSL %q: %w", raw, err)
	}
	return major, minor, nil
}
