# fix(policy): invoke server handlers only once

## Контекст

`policy.Executor` корректно поддерживает retries для произвольных операций, включая Kafka handler. Но тот же retrying executor используется серверными адаптерами `GRPCUnaryServerInterceptor` и `HTTPMiddleware`, поэтому один входящий запрос может повторно вызвать бизнес-handler.

## Проблема

Ожидаемое поведение: server-side middleware применяет timeout, rate limit, circuit breaker и concurrency limit, но выполняет пользовательский handler ровно один раз на входящий запрос.

Фактическое поведение: retryable gRPC error или HTTP 5xx повторно запускают handler. Это относится и к неидемпотентным HTTP-методам; тело запроса при повторе не восстанавливается.

Практический риск: двойные платежи, повторные записи и другие дублирующиеся side effects.

Текущий источник: `sdk/policy/policy.go`, функции `GRPCUnaryServerInterceptor` и `HTTPMiddleware`.

## Решение

- Общий `Executor.Do` и его retry-семантика не меняются.
- Каждый server adapter создаёт executor из локальной копии `Options`, в которой `Retry.Attempts` принудительно равен одному. Остальные policy options сохраняются.
- `HTTPMiddleware` вычисляет исходный `opts.Enabled()` до нейтрализации retry. Поэтому retry-only конфигурация по-прежнему включает существующую обёртку, buffering ответа и преобразование panic/5xx, но не повторяет handler.
- HTTP 5xx возвращается клиенту из единственного вызова handler.
- Retryable gRPC status возвращается клиенту с тем же gRPC code после единственного вызова handler.

Публичные сигнатуры не меняются. Конфигурация `Retry` продолжает действовать для прямого `NewExecutor` и transport-операций, но больше не означает повторный вызов server handler.

## Проверка

- Заменить тест, закрепляющий два HTTP-вызова: `POST` handler всегда отвечает 500; итоговый status — 500, счётчик вызовов — один.
- Добавить gRPC regression test: unary handler возвращает `codes.Unavailable`; interceptor возвращает `codes.Unavailable`, счётчик вызовов — один.
- Сохранить существующий `TestExecutorRetriesAndRecovers`: прямой executor по-прежнему делает настроенное количество попыток.
- Существующие проверки rate limit и других server policies остаются зелёными.

## Вне scope

- Новая idempotency-конфигурация и автоматический анализ HTTP-методов.
- Client-side retry middleware.
- Изменение retryable error classification общего executor.
- Поддержка stream interceptor, которого сейчас нет в package.

## Критерии приёмки

- HTTP и unary gRPC server handlers вызываются не более одного раза на входящий запрос при любых `Retry.Attempts`.
- HTTP buffering, panic handling и status mapping сохраняются для исходно включённой policy.
- Остальные server policies сохраняют поведение.
- Прямой `Executor.Do` продолжает поддерживать retries.
- Все существующие тесты проекта проходят.
