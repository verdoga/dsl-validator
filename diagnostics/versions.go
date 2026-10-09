package diagnostics

import (
	"fmt"

	dslversions "github.com/verdoga/dsl-validator/dsl-versions"
)

// VersionMode — один из четырёх способов выбора версий; нулевое значение неверно.
type VersionMode uint8

const (
	// VersionExact — только указанная версия.
	VersionExact VersionMode = iota + 1
	// VersionAll — любая версия из каталога поддерживаемых приложением.
	VersionAll
	// VersionFrom — указанная версия и более новые, включая границу.
	VersionFrom
	// VersionThrough — указанная версия и более старые, включая границу.
	VersionThrough
)

// VersionRule — одно правило применимости зарегистрированной диагностики.
// Любое правило действует только в пределах каталога dslversions.Supported.
type VersionRule struct {
	// Mode — обязательный способ сопоставления с версией документа.
	Mode VersionMode
	// Version — точная версия либо включительная граница; для VersionAll пуста.
	// Должна проходить Parse, но не обязана уже входить в каталог поддерживаемых.
	Version dslversions.Version
}

// validateVersionRule проверяет Mode и согласованность Version с режимом.
// VersionAll запрещает границу; остальные режимы требуют успешного Parse.
func validateVersionRule(rule VersionRule) error {
	switch rule.Mode {
	case VersionAll:
		if rule.Version != "" {
			return fmt.Errorf("режим VersionAll запрещает границу версии DSL")
		}
		return nil
	case VersionExact, VersionFrom, VersionThrough:
		if _, err := dslversions.Parse(string(rule.Version)); err != nil {
			return fmt.Errorf("неверная версия в правиле диагностики: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("неизвестный режим выбора версий DSL: %d", rule.Mode)
	}
}

// matchesVersion сопоставляет проверенное правило с поддерживаемой версией;
// true означает применимость, false — пропуск. Использует dslversions.Compare,
// а не строковый порядок; поддержку версии отдельно проверяет ForVersion.
func matchesVersion(rule VersionRule, version dslversions.Version) bool {
	switch rule.Mode {
	case VersionExact:
		return dslversions.Compare(version, rule.Version) == 0
	case VersionAll:
		return true
	case VersionFrom:
		return dslversions.Compare(version, rule.Version) >= 0
	case VersionThrough:
		return dslversions.Compare(version, rule.Version) <= 0
	default:
		return false
	}
}
