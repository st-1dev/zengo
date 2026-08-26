# fix(errx): keep internal diagnostics out of gRPC status

## Контекст

`errx.Error.GRPCStatus` формирует публичный gRPC status. Сейчас он добавляет во внешние details внутреннее сообщение, все structured fields и stack trace: metadata помещается в `google.rpc.ErrorInfo`, а сообщение и stack — в `google.rpc.DebugInfo`.

## Проблема

Ожидаемое поведение: удалённый клиент получает gRPC code, безопасный `PublicMessage` и стабильный идентификатор ошибки, но не получает внутреннюю диагностику.

Фактическое поведение: wire status раскрывает internal message, fields и stack entries независимо от окружения и уровня логирования.

Практический риск: утечка путей файлов, деталей инфраструктуры, исходных ошибок и чувствительных значений, добавленных в fields.

Текущий источник: `sdk/errx/errx.go`, методы `GRPCStatus` и `fieldMetadata`.

## Решение

- Основные code и status message остаются прежними: message берётся только из `PublicMessage`.
- `google.rpc.ErrorInfo` сохраняет `Reason` и `Domain`, чтобы удалённая сторона могла распознать статус как `errx`.
- Новые статусы не содержат internal message, structured fields и копию public message в metadata.
- `google.rpc.DebugInfo` больше не добавляется в исходящий статус.
- Локальный `*errx.Error` по-прежнему хранит internal message, fields и stack для локального логирования и диагностики.
- Декодер сохраняет чтение старых metadata keys и `DebugInfo`, чтобы принимать статусы от ещё не обновлённых peer-сервисов. Обратная совместимость нужна только для входящих старых сообщений; новые сообщения намеренно не переносят внутренние данные.

Публичные функции и типы остаются прежними. Меняется только содержимое исходящего gRPC status.

## Проверка

- Создать ошибку с уникальными secret-маркерами во внутреннем message, field и stack; убедиться, что их нет ни в status message, ни в `ErrorInfo.Metadata`, ни в других details.
- Убедиться, что status содержит ожидаемые code, `PublicMessage`, `ErrorInfo.Reason` и `ErrorInfo.Domain`.
- Убедиться, что `DebugInfo` отсутствует.
- Обновить round-trip тест: для нового статуса проверять публичную часть, не ожидать восстановления внутренней диагностики.
- Сохранить отдельный compatibility-тест декодирования вручную собранного старого статуса с legacy metadata и `DebugInfo`.

## Вне scope

- Debug opt-in по конфигурации или metadata запроса.
- Новый публичный тип для разделения public/private fields.
- Изменение локального логирования или захвата stack trace.
- Удаление legacy decoder constants.

## Критерии приёмки

- Исходящий gRPC status не содержит internal message, fields и stack trace.
- `PublicMessage`, gRPC code, `ErrorInfo.Reason` и `ErrorInfo.Domain` сохраняются.
- Локальный `*Error` не теряет внутреннюю диагностику.
- Старый wire-формат продолжает декодироваться.
- Все существующие тесты проекта проходят.
