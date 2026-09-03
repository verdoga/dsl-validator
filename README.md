# dsl-validator

Локальное Windows-приложение проверяет DSL-сценарии через закреплённый `github.com/verdoga/dslparser` и формирует JSON-отчёты рядом с исходными `.txt`.

Текущая production-библиотека `Diagnostics/` намеренно пуста и готова к регистрации диагностик DSL 1.1.

## Сборка и запуск

Из-за сетевого ограничения окружения очищенный локальный снимок зависимости расположен в `third_party/dslparser` и подключён директивой `replace`; его исходное состояние закреплено на коммите `8486082e285e179c3e506167b69f5b94b981b30c`. В обычном окружении настройте `GOPRIVATE=github.com/verdoga/*` и доступ Git без сохранения credentials в проекте.

```sh
go test ./...
GOOS=windows GOARCH=amd64 go build -trimpath -o build/validator.exe ./cmd/validator
```

Запустите `build/validator.exe`; интерфейс слушает только `http://127.0.0.1:8580/`. Выберите абсолютный каталог и режим `Автоопределение` или `1.1`, затем запустите анализ одного либо всех файлов.

## Проверка

```sh
gofmt -w .
go test ./...
go test -race ./...
go vet ./...
go list ./...
go doc dslparser
```
