set dotenv-load := true
export CGO_CFLAGS_ALLOW := "-Xpreprocessor"

# Показать доступные команды.
default:
    @just --list

# Ray Player - Разработка
dev:
    wails dev

# Ray Player - Сборка
build:
    wails build

# Проверить ffmpeg, MiniLM и локальный ONNX Runtime.
deps-check:
    go run ./cmd/ray_deps check

# Проверить ffmpeg/ffprobe и установить через системный package manager при необходимости.
deps-ffmpeg:
    go run ./cmd/ray_deps ffmpeg --install

# Скачать официальный ONNX export paraphrase-multilingual-MiniLM-L12-v2 в ignored assets/runtime.
deps-minilm:
    go run ./cmd/ray_deps minilm

# Скачать совместимый с onnxruntime_go v1.31.0 ONNX Runtime 1.26.0 в ignored assets/runtime.
deps-onnxruntime:
    go run ./cmd/ray_deps onnxruntime

# Подготовить все runtime-зависимости для локальной разработки.
deps: deps-ffmpeg deps-minilm deps-onnxruntime
    @echo "Runtime dependencies are ready"

# Ray Player - Тесты
test:
    go test -count=1 ./...

# Ray Player - Тесты с детектором гонок
test-race:
    go test -count=1 -race ./...

# Отчёт покрытия Go-пакетов.
test-cover:
    go test -count=1 -cover ./...

# Pure-JS тесты frontend state machine / EmoFlow.
test-frontend:
    npm --prefix frontend test

# Полный локальный quality gate перед коммитом.
test-all: test test-race test-cover test-frontend
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
