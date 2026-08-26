# testwarden

[![CI](https://github.com/human-horizon/testwarden/actions/workflows/ci.yml/badge.svg)](https://github.com/human-horizon/testwarden/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/human-horizon/testwarden)](https://goreportcard.com/report/github.com/human-horizon/testwarden)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

CLI watchdog for test coverage and over-mocking in Go and TypeScript projects.

## Что делает

- Следит за покрытием unit-тестов в Go (`coverage.out`) и TypeScript (`lcov.info`).
- Детектит **over-mocking** в unit-тестах через real Go AST (моки на `database/sql`, `net/http`, `pg`, `fs` и т.д.).
- Детектит **coverage gaps** — строки, не покрытые ни unit, ни integration тестами.
- Опционально **чинит** найденные проблемы через локальный OpenAI-compatible LLM (Ollama, LM Studio, llama.cpp server).
- **Streaming AI responses** через SSE + **retry с exponential backoff**.
- **Incremental cache** по sha256 файлов (`.testwarden/cache/manifest-{go,ts}.json`).
- **Interactive TUI** через bubbletea (spinner, progress bar, viewport).
- CI-friendly: `exit 1` при нарушениях, `--json` для парсинга, `--no-tui` для логов.
- **Git pre-commit hook installer** (`testwarden init-hooks`).

## Установка

```bash
go install github.com/human-horizon/testwarden@latest
```

Или скачай бинарь из [Releases](https://github.com/human-horizon/testwarden/releases).

## Использование

```bash
# Создать конфиг с дефолтами
testwarden init

# Проверить проект
testwarden check

# Проверить только Go
testwarden check --lang go

# JSON-вывод для CI
testwarden check --json

# Отключить TUI (plain text для логов)
testwarden --no-tui check

# Показать что AI починил бы, без записи
testwarden fix --dry-run

# Реально починить через AI
testwarden fix

# Установить git pre-commit hook
testwarden init-hooks
```

## Конфигурация (`.testwarden.yml`)

```yaml
coverage:
  unit_threshold: 80
  integration_gap_threshold: 5
  unit_path: coverage.out
  integration_path: integration-coverage.out
  unit_command: "go test -coverprofile=coverage.out ./..."
  integration_command: "go test -tags=integration -coverprofile=integration-coverage.out ./..."

mocks:
  detect_overmocking: true
  real_dependencies:
    go: [database/sql, net/http, os]
    typescript: [fs, http, pg, mysql2]

ai:
  endpoint: "http://localhost:11434/v1"
  api_key: ""
  model: "qwen2.5-coder"
  timeout: 120
  max_tokens: 4096

languages: [go, typescript]
```

## Запуск тестов

```bash
go test ./...
```

## Стек

- Go 1.26+
- [spf13/cobra](https://github.com/spf13/cobra) — CLI
- [gopkg.in/yaml.v3](https://gopkg.in/yaml.v3) — конфиг
- [sashabaranov/go-openai](https://github.com/sashabaranov/go-openai) — OpenAI-compatible клиент
- [charmbracelet/bubbletea](https://github.com/charmbracelet/bubbletea) — TUI
- [golang.org/x/sync/errgroup](https://pkg.go.dev/golang.org/x/sync/errgroup) — parallel analysis

## Лицензия

MIT