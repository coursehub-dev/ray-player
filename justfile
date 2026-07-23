set dotenv-load := true

# Показать доступные команды.
default:
    @just --list

# Ray Player - Разработка
dev:
    CGO_CFLAGS_ALLOW="-Xpreprocessor" wails dev

# Ray Player - Сборка
build:
    CGO_CFLAGS_ALLOW="-Xpreprocessor" wails build

# Ray Player - Тесты
test:
    CGO_CFLAGS_ALLOW="-Xpreprocessor" go test -count=1 -v ./...

# Ray Player - Тесты с детектором гонок
test-race:
    CGO_CFLAGS_ALLOW="-Xpreprocessor" go test -count=1 -race -v ./...

# Ray Player - Очистка кэша тестов и запуск
test-clean:
    go clean -testcache && CGO_CFLAGS_ALLOW="-Xpreprocessor" go test -count=1 -v ./...

# Ray Player - Аудит аудио в указанной папке
audio-probe AUDIO_DIR:
    go run ./cmd/audio_probe_batch -config ./docs/audio_probe_config.json -audio-dir "{{AUDIO_DIR}}" -out "{{AUDIO_DIR}}/audio_probe_report.json"

# Ray Player - Очистка артефактов сборки
clean:
    rm -rf build/bin frontend/dist
