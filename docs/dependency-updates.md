# Dependency update policy

## Цель

Обновления должны быть небольшими, воспроизводимыми и независимо
принимаемыми. Не объединяйте в один PR зависимости только потому, что они
используют один package manager.

## Группы Dependabot

- `svelte-toolchain`: Svelte, Svelte Vite plugin и Vite, только minor/patch.
- `frontend-quality-tooling`: Biome, только minor/patch.
- `sqlite-stack`: `modernc.org/sqlite` и требуемый им `modernc.org/libc`.
- `wails-stack`: модули Wails v2 и связанные модули Lea Anthony.
- `golang-x-routine`: minor/patch-обновления связанного набора `golang.org/x/*`.
- `actions-routine`: только официальные `actions/*`, minor/patch.

Все остальные зависимости остаются отдельными PR. Major-обновления не
игнорируются, но создаются отдельно и требуют ручной миграции.

Security updates не группируются с обычными version updates: их нужно
рассматривать срочно и отдельно.

## Dependency Review в Pull Request

`actions/dependency-review-action` использует GitHub Dependency Graph и
REST dependency-review API. Это серверная возможность репозитория: наличие
`go.mod`, `go.sum` и `package-lock.json` само по себе её не включает.

Workflow сохраняет один стабильный required check `CI / dependency-review`:

- при repository variable `DEPENDENCY_REVIEW_ENABLED=true` запускается
  строгий GitHub Dependency Review с `fail-on-severity: moderate`;
- если переменная отсутствует или равна `false`, action не вызывается, потому
  что на репозитории без Dependency Graph он падает до анализа зависимостей;
- fallback выполняет `go mod download`, `go mod verify` и `npm audit` по
  production dependency graph с порогом `moderate`. Он не исполняет npm
  lifecycle scripts.

Fallback является защитой от инфраструктурного false-negative/false-red CI,
но не полной заменой GitHub Dependency Review: он проверяет текущее состояние,
а не diff зависимостей между base/head PR. Поэтому для основного публичного
репозитория Dependency Graph нужно включить.

Однократная настройка владельцем репозитория:

1. GitHub → `Settings` → `Security` / `Advanced Security` → включить
   **Dependency graph**.
2. Дождаться построения graph; обычно это занимает несколько минут.
3. GitHub → `Settings` → `Secrets and variables` → `Actions` → `Variables` →
   создать `DEPENDENCY_REVIEW_ENABLED=true`.
4. Перезапустить PR workflow. После этого инфраструктурная недоступность
   Dependency Graph снова считается ошибкой, а уязвимость уровня
   `moderate` и выше блокирует PR.

Не используйте для этого job `continue-on-error` или `warn-only`: они также
скроют реальные vulnerability findings, а не только отсутствие серверной
функции GitHub.

## Новые advisory без изменения исходников

`govulncheck` использует обновляемую Go Vulnerability Database. Поэтому
ранее зелёный commit может начать падать после публикации нового advisory:
это новый результат проверки безопасности, а не недетерминированная сборка.

При таком падении:

1. проверьте, что отчёт содержит достижимый symbol trace;
2. обновите затронутый модуль как минимум до версии из `Fixed in`;
3. выполните `go mod tidy`, `just deps-verify`, `just test-all` и
   `just security-check`;
4. не подавляйте advisory и не помечайте security job как
   `continue-on-error`.

Для модулей с версиями `v0.x` исправление может быть minor-обновлением,
поэтому routine-группа `golang.org/x/*` принимает minor и patch. Security
updates при этом остаются отдельными срочными PR.

Workflow `Security / govulncheck` запускается для push, pull request,
вручную и ежедневно по расписанию. Он должен быть required check для
защищённой ветки `main`.

## Обязательные проверки

```bash
npm ci --prefix frontend --strict-peer-deps
npm --prefix frontend run deps:check
just deps-verify
just test-all
just security-check
```

Для нативных или CGO-зависимостей дождитесь обеих matrix-сборок:

- macOS arm64;
- Windows amd64.

## Generated files

Не редактируйте вручную:

- `frontend/package-lock.json`;
- `go.sum`;
- `frontend/package.json.md5`;
- `frontend/wailsjs/*`.

Lock-файл frontend создаётся npm, закреплённым в `packageManager`.
После изменения `frontend/package.json` обновите MD5:

```bash
# Linux
md5sum frontend/package.json | awk '{print $1}' > frontend/package.json.md5

# macOS
md5 -q frontend/package.json > frontend/package.json.md5
```

Go graph обновляется так:

```bash
go get <module>@<version>
go mod tidy
go mod verify
```

## Хрупкие контракты

### SQLite

`modernc.org/sqlite` должен использовать именно ту версию
`modernc.org/libc`, которая указана в `go.mod` выбранной версии SQLite.
Не заменяйте её просто на самый новый libc.

Проверка:

```bash
./scripts/check-sqlite-libc-version.sh
```

### Wails

CLI должен совпадать с `github.com/wailsapp/wails/v2` из `go.mod`:

```bash
just wails-install
```

Не используйте `@latest` в CI или release workflow.

### ONNX Runtime

`github.com/yalue/onnxruntime_go` и поставляемая приложением нативная
ONNX Runtime образуют отдельный runtime contract. Их обновление всегда
делается отдельным PR с model/runtime smoke tests.

## Порядок принятия bot PR

1. Проверить release notes, manifest diff и lock/checksum diff.
2. Убедиться, что PR не содержит посторонних обновлений.
3. Дождаться всех required checks.
4. Для dependency PR выполнить `just security-check`.
5. При конфликте с `main` разрешить Dependabot обновить ветку или
   использовать merge queue; не объединять два независимых PR вручную.
6. Major, Wails, SQLite и ONNX обновления принимать только вручную.

## Полезные команды Dependabot

В комментарии bot PR:

```text
@dependabot rebase
@dependabot recreate
@dependabot ignore this major version
```

`recreate` нужен только когда PR должен быть заново сформирован по новой
группировке или обновлённому manifest contract.
