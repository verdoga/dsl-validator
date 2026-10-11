package validator

import (
	"fmt"

	"github.com/verdoga/dsl-validator/diagnostics"
	dslversions "github.com/verdoga/dsl-validator/dsl-versions"
	"github.com/verdoga/dsl-validator/model"
)

// Prepare определяет версию по Document.DSLVersion и вызывает Registry.ForVersion
// с её значением как dslversions.Version; nil передаётся как пустая версия.
// Не извлекает версию повторно из Raw и не подставляет текущую версию каталога.
// Неподдерживаемая, некорректная или неизвестная версия даёт error до полного
// контроля модели, копирования строк, создания Processing и вызовов Check.
// Отсутствие подходящих диагностик также даёт error с версией документа.
// При пригодной версии и непустой выборке вызывает CheckModel ровно один раз
// и возвращает checks в порядке V-кодов. nil result/registry дают error.
// При любой ошибке checks=nil. Модель и реестр не изменяет, I/O не выполняет.
func Prepare(result *model.Result, registry *diagnostics.Registry) (checks []diagnostics.Definition, err error) {
	if result == nil {
		return nil, fmt.Errorf("модель отсутствует")
	}
	if registry == nil {
		return nil, fmt.Errorf("реестр диагностик отсутствует")
	}
	var version dslversions.Version
	if result.Document.DSLVersion != nil {
		version = dslversions.Version(*result.Document.DSLVersion)
	}
	checks, err = registry.ForVersion(version)
	if err != nil {
		return nil, err
	}
	if len(checks) == 0 {
		return nil, fmt.Errorf("для версии DSL %q нет подходящих диагностик", version)
	}
	if err := CheckModel(result); err != nil {
		return nil, fmt.Errorf("целостность модели: %w", err)
	}
	return checks, nil
}
