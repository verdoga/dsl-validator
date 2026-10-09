// Package diagnostics задаёт универсальный реестр, регистрацию и ограниченное
// API проверок с выбором по версии DSL. Он не импортирует model, validator
// или checkers, не содержит определений P/V-диагностик и не создаёт записей JSON.
// Интерфейсы ниже — граница доступа к строкам и приёма срабатываний;
// конкретный реестр интерфейсом не оборачивается.
package diagnostics

// Code — машинный код; новые проверки валидатора используют V001 и далее.
// Сам тип допускает прежние коды других инструментов для совместимости model.
type Code string

// Severity — существующий тип уровня из JSON-контракта 1.0.
type Severity string

const (
	// SeverityError — ошибка, устанавливающая hasErrors.
	SeverityError Severity = "error"
	// SeverityWarning — проблема; машинное значение warning сохраняется.
	SeverityWarning Severity = "warning"
	// SeverityRecommendation — рекомендация.
	SeverityRecommendation Severity = "recommendation"
)

// Scope — существующий тип области из JSON-контракта 1.0.
type Scope string

const (
	// ScopeElement — срабатывание на точном диапазоне существующего элемента.
	ScopeElement Scope = "element"
	// ScopeLine — срабатывание внутри одной физической строки.
	ScopeLine Scope = "line"
	// ScopeBlock — срабатывание на диапазоне, допускающем несколько строк.
	ScopeBlock Scope = "block"
	// ScopeDocument — срабатывание на документе, в том числе без позиции.
	ScopeDocument Scope = "document"
)
