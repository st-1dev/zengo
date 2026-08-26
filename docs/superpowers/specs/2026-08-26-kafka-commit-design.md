# fix(kafka): preserve messages when handlers fail

## Контекст

`consumerGroupHandler.ConsumeClaim` передаёт каждое сообщение в `consumeMessage`. Сейчас ошибка обработчика после выполнения настроенной retry-политики только журналируется, а затем offset безусловно помечается через `sarama.ConsumerGroupSession.MarkMessage`.

## Проблема

Ожидаемое поведение: сообщение подтверждается только после успешной бизнес-обработки либо после явно выбранной обработки poison pill.

Фактическое поведение: ошибка handler не доходит до Sarama, offset помечается обработанным и событие может быть потеряно без возможности повторной доставки.

Практический риск: необратимая потеря бизнес-событий после временной или постоянной ошибки обработчика.

Текущий источник: `sdk/transport/queue/kafka/kafka.go`, методы `ConsumeClaim` и `consumeMessage`.

## Решение

- `consumeMessage` возвращает ошибку выполнения handler.
- При успешной обработке сообщение помечается ровно один раз.
- При ошибке handler сообщение не помечается, а `ConsumeClaim` возвращает эту ошибку Sarama. Следующий consumer-group session сможет получить сообщение повторно.
- Ошибка декодирования envelope сохраняет текущее poison-pill поведение: ошибка журналируется, сообщение помечается и обработка claim продолжается. В рамках этого MR политика для malformed-сообщений не меняется.
- Если executor не задан, сохраняется текущий fallback на executor с пустыми options.

Публичный Go API не меняется: затрагиваются только внутренние методы consumer group handler. Наблюдаемая семантика намеренно меняется с потери события на повторную доставку после handler error.

## Проверка

Тесты размещаются во внутреннем package `kafka`, чтобы напрямую проверить `consumerGroupHandler` с минимальными fake-реализациями `sarama.ConsumerGroupSession` и claim.

- Успешный handler: один вызов handler, один `MarkMessage`, `ConsumeClaim` завершается без ошибки.
- Ошибка handler: один завершившийся вызов executor/handler, ноль `MarkMessage`, `ConsumeClaim` возвращает ошибку.
- Некорректный envelope: handler не вызывается, сообщение помечается один раз, затем claim завершается штатно.

## Вне scope

- DLQ, quarantine topic и новый callback для poison pill.
- Бесконечные retries внутри одного claim.
- Изменение публичных типов Kafka transport.
- Изменение политики декодирования malformed envelope.

## Критерии приёмки

- Ни один путь с handler error не вызывает `MarkMessage`.
- Успешный путь и текущий decode-error путь отмечают сообщение ровно один раз.
- Ошибка handler возвращается из `ConsumeClaim`.
- Все существующие тесты проекта проходят без изменения публичного API.
