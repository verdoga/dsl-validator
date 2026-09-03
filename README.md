# dsl-validator

Локальное Windows-приложение проверяет DSL-сценарии через закреплённый `github.com/verdoga/dslparser` и формирует JSON-отчёты рядом с исходными `.txt`.

Текущая production-библиотека `Diagnostics/` намеренно пуста и готова к регистрации диагностик DSL 1.1.

## Сборка и запуск

Парсер подключён непосредственно как внешний модуль на коммите `8486082e285e179c3e506167b69f5b94b981b30c`. Для закрытого репозитория настройте `GOPRIVATE=github.com/verdoga/*` и доступ Git без сохранения учётных данных в проекте.

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
