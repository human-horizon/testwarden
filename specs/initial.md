# TestWarden — CLI для надзора за тестовым покрытием

## Контекст

В проектах на Go и TypeScript систематически возникают три проблемы:

1. **Покрытие unit-тестов тихо падает** ниже приемлемого порога — никто не замечает до продакшена.
2. **Интеграционные тесты дублируют unit-тесты**, а реальные «хвосты» (ветки, не покрытые ни unit, ни integration) остаются непокрытыми.
3. **Over-mocking**: моки прорастают на реальных зависимостях (БД, HTTP, файловая система), превращая «интеграционные» тесты в ещё одни unit-тесты.

TestWarden — единый CLI, который **детектирует** эти проблемы и **автоматически чинит** их через локальный OpenAI-compatible LLM (например, Ollama, LM Studio, llama.cpp server).

## Цель

Go CLI `testwarden`, который в проектах на Go и/или TypeScript:

1. Измеряет покрытие unit-тестов и сравнивает с настраиваемым порогом.
2. Детектит **coverage gaps** — ветки/строки, не покрытые ни unit, ни integration тестами.
3. Детектит **over-mocking** в unit-тестах (моки на реальных I/O зависимостях).
4. В режиме `--fix` отправляет проблемы в локальный OpenAI-compatible endpoint и применяет сгенерированные unified diff-патчи с автоматическим rollback при падении тестов.

## Что изменится

Создаётся новый проект `~/Space/Projects/HumanHorizon/testwarden/` с нуля.

## Детали реализации

### Команды CLI

- `testwarden init` — создаёт `.testwarden.yml` с дефолтами в cwd.
- `testwarden check [--lang go|ts]` — анализирует проект, печатает отчёт, **exit 1** при наличии нарушений, **exit 0** если всё ок или dry-run.
- `testwarden fix [--dry-run] [--lang go|ts]` — то же что `check`, плюс для каждой проблемы вызывает AI и применяет патч.

### Конфигурация — `.testwarden.yml`

```yaml
coverage:
  unit_threshold: 80              # минимальный % покрытия unit-тестами
  integration_gap_threshold: 5    # максимальный % непокрытых веток (gap)

mocks:
  detect_overmocking: true
  # Список реальных зависимостей, которые НЕЛЬЗЯ мокать в unit-тестах
  real_dependencies:
    go:
      - "database/sql"
      - "net/http"
      - "os"
    typescript:
      - "fs"
      - "http"
      - "pg"
      - "mysql2"

ai:
  endpoint: "http://localhost:11434/v1"   # OpenAI-compatible base URL
  api_key: ""                            # пусто для локального
  model: "qwen2.5-coder"
  timeout: 120                           # секунд
  max_tokens: 4096

languages: [go, typescript]
```

### Архитектура

```
testwarden/
├── cmd/
│   ├── root.go            # корневая команда
│   ├── check.go           # команда check
│   ├── fix.go             # команда fix
│   └── init.go            # команда init
├── internal/
│   ├── config/            # загрузка .testwarden.yml
│   ├── coverage/
│   │   ├── go.go          # парсер coverage.out (Go)
│   │   └── ts.go          # парсер lcov.info (TS)
│   ├── mocks/
│   │   ├── go.go          # AST-анализ моков (mocks.Mock, gomock, testify/mock)
│   │   └── ts.go          # AST-анализ моков (jest.mock, sinon)
│   ├── ai/
│   │   └── client.go      # OpenAI-compatible клиент
│   ├── patcher/
│   │   ├── apply.go       # применение unified diff
│   │   └── rollback.go    # откат при падении тестов
│   └── report/
│       └── report.go      # форматирование отчёта (text + JSON)
├── go.mod
├── go.sum
└── .testwarden.yml.example
```

### Поток `check`

1. Загрузить `.testwarden.yml` (или defaults).
2. Определить язык проекта (по наличию `go.mod` / `package.json`).
3. Запустить coverage tool (`go test -coverprofile` / `nyc --reporter=lcov`).
4. Распарсить coverage → извлечь % покрытых строк/веток.
5. Просканировать unit-тесты на over-mocking.
6. Вычислить coverage gap (строки, не покрытые ни unit, ни integration).
7. Собрать список нарушений.
8. Напечатать отчёт (text по умолчанию, `--json` для CI).
9. **exit 1** если есть нарушения, иначе **exit 0**.

### Поток `fix`

1. Выполнить шаги 1–7 из `check`.
2. Если нарушений нет — выйти с exit 0.
3. Для каждого нарушения:
   - Собрать контекст: путь файла, проблемный код, правило из конфига.
   - Отправить в AI endpoint (system + user промпт).
   - Получить unified diff.
   - **Сделать backup файла** (в `.testwarden/backup/`).
   - Применить patch.
   - Прогнать тесты (`go test` / `npm test`).
   - Если тесты упали — **откатить** из backup, перейти к следующему нарушению.
4. Напечатать итог: сколько починено, сколько пропущено.

### AI-промпт (шаблон)

**System:**
```
You are a senior test engineer. You receive a problem description and a code file.
Respond ONLY with a unified diff (--- a/path, +++ b/path, @@ hunks @@).
Do not add explanations. Minimal change. Preserve formatting.
```

**User:**
```
Problem: <тип нарушения>
Rule: <правило из конфига>
File: <путь>
Language: <go|typescript>

<содержимое файла>
```

### Зависимости (Go modules)

- `github.com/spf13/cobra` — CLI
- `gopkg.in/yaml.v3` — конфиг
- `github.com/sashabaranov/go-openai` — OpenAI-compatible клиент
- Стандартная библиотека Go для остального (AST, exec, diff).

## Критерии приёмки

- [ ] `testwarden init` создаёт валидный `.testwarden.yml` с дефолтами.
- [ ] `testwarden check` в Go-проекте с покрытием 50% возвращает **exit 1** и понятный отчёт.
- [ ] `testwarden check` в Go-проекте с покрытием 95% возвращает **exit 0**.
- [ ] `testwarden check` детектит мок `database/sql.DB` в unit-тесте как over-mocking (когда `detect_overmocking: true`).
- [ ] `testwarden fix --dry-run` показывает какие файлы будут изменены, **ничего не пишет на диск**.
- [ ] `testwarden fix` отправляет запрос на OpenAI-compatible endpoint и применяет полученный unified diff.
- [ ] Если после применения патча тесты падают — файл откатывается из backup.
- [ ] Конфиг читается из `.testwarden.yml` в cwd; если файла нет — используются defaults.
- [ ] `--json` флаг выдаёт отчёт в machine-readable формате.
- [ ] Линтер (`golangci-lint`) без ошибок.
- [ ] Unit-тесты покрывают ключевые модули (`config`, `coverage/go`, `mocks/go`, `patcher`).
