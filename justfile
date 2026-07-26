set dotenv-load := true
export CGO_CFLAGS_ALLOW := "-Xpreprocessor"

# Windows: без sh (Git Bash). macOS/Linux не затрагиваются — там остаётся sh.
# Bypass — только для процесса just, системный ExecutionPolicy не меняется.
[windows]
set shell := ["powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command"]

# Показать доступные команды.
default:
    @just --list

# Установить Wails CLI той же версии, что и модуль в go.mod.
wails-install:
    ./scripts/install-wails-cli.sh

# Ray Player - Разработка
dev:
    wails dev

# Windows: CGO + MinGW (WinLibs) PATH, затем wails dev.
# MinGW: PATH / RAY_MINGW_BIN / WinGet BrechtSanders.WinLibs*.
# macOS/Linux: используйте `just dev`.
dev-win:
    ./scripts/dev.ps1

# Ray Player - Обычная сборка (использует уже доступные runtime/model assets).
build:
    wails build

# Сборка с runtime/model ассетами из пользовательской папки данных внутри артефакта.
build-portable: deps
    wails build
    go run ./cmd/ray_deps stage --build-dir build/bin

# Проверить ffmpeg, модели и реально выбранный ONNX Runtime (локальный или системный).
deps-check:
    go run ./cmd/ray_deps check

# Проверить ffmpeg/ffprobe и при отсутствии скачать portable-бинарники в пользовательские ассеты.
deps-ffmpeg:
    go run ./cmd/ray_deps ffmpeg --install

# Скачать официальный ONNX export paraphrase-multilingual-MiniLM-L12-v2 в пользовательские ассеты.
deps-minilm:
    go run ./cmd/ray_deps minilm

# Скачать совместимый ONNX Runtime 1.26.0 в пользовательские ассеты (для portable/staging).
deps-onnxruntime:
    go run ./cmd/ray_deps onnxruntime

# Скачать полный runtime-набор Essentia в управляемую папку приложения.
deps-essentia:
    go run ./cmd/ray_deps essentia

# Подготовить все runtime-зависимости и сразу выполнить реальный smoke-test.
deps:
    go run ./cmd/ray_deps ffmpeg --install
    go run ./cmd/ray_deps onnxruntime
    go run ./cmd/ray_deps minilm
    go run ./cmd/ray_deps essentia
    go run ./cmd/ray_deps check
    @echo "Runtime dependencies are ready and verified"

# Проверить целостность Go-модулей и связку modernc SQLite/libc.
deps-verify:
    go mod verify
    ./scripts/check-sqlite-libc-version.sh

# Проверить достижимые уязвимости Go-кода закреплённой версией govulncheck.
security-check: frontend-build
    ./scripts/run-govulncheck.sh

# Форматировать Go и frontend вручную.
format:
    gofmt -w $(git ls-files '*.go')
    npm --prefix frontend run format

# Немутабельная проверка Go formatting и frontend format/lint.
format-check:
    ./scripts/check-go-format.sh
    ./scripts/check-frontend-quality.sh

# Подключить безопасный локальный hook без npm install и сетевых действий.
hooks:
    git config core.hooksPath .githooks
    @echo "Git hooks enabled from .githooks"

# Ray Player - Тесты
test: frontend-build
    go test -count=1 ./...

# Ray Player - Тесты с детектором гонок
test-race: frontend-build
    go test -count=1 -race ./...

# Отчёт покрытия Go-пакетов.
test-cover: frontend-build
    go test -count=1 -cover ./...

# Pure-JS тесты frontend state machine / UI contracts / EmoFlow.
test-frontend:
    npm --prefix frontend test

# Проверка production-сборки Svelte/Vite.
frontend-build:
    npm --prefix frontend run build
    ./scripts/check-frontend-dist.sh

# Статический анализ Go.
vet: frontend-build
    go vet ./...

# Полный локальный quality gate перед коммитом.
test-all: format-check deps-verify test-frontend frontend-build vet test test-cover
    @echo "All quality gates passed"

# Ray Player - Очистка кэша тестов и запуск
test-clean: frontend-build
    go clean -testcache
    go test -count=1 ./...

# Ray Player - Аудит аудио в указанной папке
audio-probe AUDIO_DIR:
    go run ./cmd/audio_probe_batch -config ./docs/audio_probe_config.json -audio-dir "{{AUDIO_DIR}}" -out "{{AUDIO_DIR}}/audio_probe_report.json"

# Ray Player - Очистка артефактов сборки
clean:
    rm -rf build/bin frontend/dist
