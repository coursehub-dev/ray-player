# Как внести вклад в Ray Player

Спасибо за интерес к проекту. Ray Player — десктопное приложение на Go, Wails 2 и Svelte. ML-анализ выполняется локально через ONNX Runtime и модели Essentia.

## 1. Что установить

Обязательные инструменты:

- Go версии, указанной в `go.mod`;
- Node.js версии, указанной в `.node-version`;
- npm версии, указанной в `frontend/package.json` в поле `packageManager`;
- Wails CLI той же major/minor-ветки, что и зависимость `github.com/wailsapp/wails/v2`;
- `just`;
- системные зависимости Wails для вашей ОС.

Установка Wails CLI:

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0
wails doctor
```

Клонирование и подготовка:

```bash
git clone https://github.com/coursehub-dev/ray-player.git
cd ray-player
npm ci --prefix frontend
just hooks
just deps
```

`just deps` загружает управляемые FFmpeg, ONNX Runtime, MiniLM и Essentia-модели в пользовательскую папку Ray Player и выполняет smoke-test. Не добавляйте локальные абсолютные пути в коммиты.

## 2. Перед началом работы

1. Найдите или создайте issue с описанием ошибки или изменения.
2. Обновите основную ветку.
3. Создайте отдельную ветку от актуальной `main`.

```bash
git switch main
git pull --ff-only
git switch -c fix/short-description
```

Используйте понятные префиксы: `fix/`, `feat/`, `docs/`, `test/`, `ci/`.

## 3. Правила изменений

- Не корректируйте ML-результаты по названию файла, исполнителю, словам в title или заранее зашитым жанрам. Исправления должны находиться в аудиопайплайне, контракте модели, агрегации выходов или общей калибровке.
- Не коммитьте приватные музыкальные файлы, пользовательскую БД, логи, токены и абсолютные пути.
- Не меняйте сгенерированные Wails bindings вручную как единственный источник изменения. Обновите Go API и запустите `just dev`, чтобы bindings были перегенерированы.
- Для новой бизнес-логики сначала добавляйте тест, воспроизводящий требуемое поведение.
- Не добавляйте сетевые действия в git hooks. Pre-commit проекта только проверяет `gofmt`.
- Новые внешние зависимости должны иметь понятную необходимость и совместимую лицензию.

Frontend toolchain `svelte`, `@sveltejs/vite-plugin-svelte` и `vite`
считается одной совместимой единицей:

- не обновляйте один из этих пакетов на новый major отдельно;
- не используйте `--force` или `--legacy-peer-deps`;
- после изменения зависимостей полностью перегенерируйте lock-файл теми
  версиями Node/npm, которые закреплены проектом;
- обновите `frontend/package.json.md5`.

```bash
cd frontend
rm -rf node_modules package-lock.json
npm install --package-lock-only --strict-peer-deps
npm ci --strict-peer-deps
npm run deps:check
cd ..
```

## 4. Разработка

Запуск приложения:

```bash
just dev
```

Эта команда запускает Wails dev-mode, frontend и заново генерирует bindings между Go и Svelte.

Полезные команды:

```bash
just test                 # Go-тесты
just test-frontend        # pure JS-тесты
just frontend-build       # production-сборка Svelte
just vet                  # go vet
just format               # gofmt и Biome
just deps-check           # smoke-test runtime и моделей
just audio-probe /path/to/audio
```

Для изменений ML приложите обезличенный probe-отчёт или сводку до/после. Минимально проверьте:

- число успешно проанализированных файлов;
- размер аудио-эмбеддинга — 1280;
- отсутствие насыщения/деградации выходов модели;
- устойчивость genre primary/label/tags;
- отсутствие пересчёта кластеров на каждом треке пакетной индексации.

## 5. Перед коммитом

```bash
just format
just test-all
```

Race detector запускается отдельно, потому что зависит от CGO и платформы:

```bash
just test-race
```

Проверьте diff:

```bash
git diff --check
git status --short
```

Коммиты должны быть небольшими и логически цельными. Рекомендуемый формат:

```text
fix: keep accepted Essentia genre as primary
feat: add managed Essentia repair
ci: publish native desktop artifacts
```

## 6. Pull Request

```bash
git push -u origin fix/short-description
```

Создайте Pull Request в `main`. В описании укажите:

- проблему и причину;
- выбранное решение;
- какие тесты запускались;
- платформы, на которых проверена сборка;
- влияние на модели, БД или миграции;
- скриншоты для заметных UI-изменений.

PR должен пройти GitHub Actions. Не объединяйте PR с красным CI. Один PR не должен одновременно содержать несвязанные ML-, UI- и инфраструктурные рефакторинги.

## 7. Релизы

Релизная сборка запускается тегом вида `v1.2.3`:

```bash
git tag -s v1.2.3 -m "Ray Player v1.2.3"
git push origin v1.2.3
```

Workflow собирает Windows amd64 и macOS arm64 нативно, добавляет portable runtime/model assets и публикует архивы в GitHub Release. macOS-артефакт без настроенных Apple signing secrets остаётся неподписанным; это нужно явно указать в release notes.

## 8. Лицензия

Отправляя вклад, вы соглашаетесь распространять его на условиях GNU GPL version 3, указанных в `LICENSE.md`. Сторонние модели и runtime-библиотеки сохраняют собственные лицензии и notices.
