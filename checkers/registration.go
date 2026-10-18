// Package checkers содержит автономные диагностики и единственный
// каталог функций их регистрации.
// Пакет зависит только от diagnostics; модели документа и файлового I/O нет.
// Используется единое имя checkers; отдельный пакет checker не создаётся.
package checkers

import "github.com/verdoga/dsl-validator/diagnostics"

// RegistrationCheckers возвращает единственную карту-каталог вида
// стабильное имя регистрации -> функция регистрации.
// Элементы добавляются в литерал карты внутри этой функции;
// это не реестр диагностик и не выполняемая во время init регистрация.
// Карта принадлежит вызывающему коду и передаётся в diagnostics.NewRegistry.
// Функции регистрации имеют тип diagnostics.RegistrationFunc, вызывают
// Registry.Register и передают ему Definition с конкретной Check.
func RegistrationCheckers() map[string]diagnostics.RegistrationFunc {
	return map[string]diagnostics.RegistrationFunc{
		"V001": registrationV001,
		"V002": registrationV002,
		"V003": registrationV003,
		"V004": registrationV004,
		"V005": registrationV005,
		"V006": registrationV006,
		"V008": registrationV008,
		"V010": registrationV010,
		"V011": registrationV011,
		"V012": registrationV012,
		"V014": registrationV014,
		"V016": registrationV016,
		"V017": registrationV017,
		"V019": registrationV019,
		"V021": registrationV021,
		"V022": registrationV022,
		"V024": registrationV024,
		"V026": registrationV026,
		"V027": registrationV027,
		"V029": registrationV029,
		"V031": registrationV031,
		"V032": registrationV032,
		"V034": registrationV034,
		"V036": registrationV036,
		"V037": registrationV037,
		"V038": registrationV038,
		"V042": registrationV042,
		"V043": registrationV043,
		"V044": registrationV044,
		"V046": registrationV046,
		"V048": registrationV048,
	}
}
