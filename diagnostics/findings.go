package diagnostics

// Findings принимает только срабатывания текущего Check; прочитать прежние
// срабатывания, назначить ID, Source, Code, Severity либо Fatal через него нельзя.
// Позиции передаются числами, чтобы не копировать model.Position/Location
// и не создавать цикл model -> diagnostics -> model.
type Findings interface {
	// Add фиксирует диапазон [start,end) по строкам и Unicode-колонкам с 1.
	// Все четыре нуля означают отсутствие Location только для ScopeDocument.
	// Для ScopeElement диапазон обязан совпасть с существующим элементом;
	// для ScopeLine обе строки совпадают. message="" выбирает Definition.Message.
	// occurrence — положительный локальный номер срабатывания, не JSON ID;
	// при ошибке равен 0. Первая ошибка запрещает дальнейшее накопление и делает
	// весь Check неуспешным, даже если проверка ошибочно проигнорировала её.
	Add(startLine, startColumn, endLine, endColumn int, message string) (occurrence int, err error)
	// AddRelated добавляет связанный диапазон к ранее полученному occurrence
	// этого Check. Проверяет координаты, запрещает нулевой диапазон без позиции,
	// повтор и совпадение с основной Location. Не создаёт Element.ErrorIDs.
	AddRelated(occurrence, startLine, startColumn, endLine, endColumn int) error
}
