set dotenv-load := true
export CGO_CFLAGS_ALLOW := "-Xpreprocessor"

# Показать доступные команды.
default:
    @just --list

# Ray Player - Разработка
dev:
    wails dev

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


# Форматировать Go и frontend вручную.
format:
    gofmt -w $(git ls-files '*.go')
    npm --prefix frontend run format

# Немутабельная проверка форматирования для CI и pre-commit.
format-check:
    ./scripts/check-go-format.sh
    npm --prefix frontend run check

# Подключить безопасный локальный hook без npm install и сетевых действий.
hooks:
    git config core.hooksPath .githooks
    @echo "Git hooks enabled from .githooks"

# Ray Player - Тесты
test:
    go test -count=1 ./...

# Ray Player - Тесты с детектором гонок
test-race:
    go test -count=1 -race ./...

# Отчёт покрытия Go-пакетов.
test-cover:
    go test -count=1 -cover ./...

# Pure-JS тесты frontend state machine / UI contracts / EmoFlow.
test-frontend:
    npm --prefix frontend test

# Проверка production-сборки Svelte/Vite.
frontend-build:
    npm --prefix frontend run build

# Статический анализ Go.
vet:
    go vet ./...

# Полный локальный quality gate перед коммитом.
test-all: format-check vet test test-cover test-frontend frontend-build
    @echo "All quality gates passed"

# Ray Player - Очистка кэша тестов и запуск
test-clean:
    go clean -testcache
    go test -count=1 ./...

# Ray Player - Аудит аудио в указанной папке
audio-probe AUDIO_DIR:
    go run ./cmd/audio_probe_batch -config ./docs/audio_probe_config.json -audio-dir "{{AUDIO_DIR}}" -out "{{AUDIO_DIR}}/audio_probe_report.json"

# Ray Player - Очистка артефактов сборки
clean:
    rm -rf build/bin frontend/dist
