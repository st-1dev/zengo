# fix(tlsconfig): reject unknown client-auth modes

## Контекст

`tlsconfig.ServerOptions.ClientAuth` — строковый `ClientAuthMode`. Конструктор server TLS config знает только `none`, `verify_if_given` и `require_and_verify`, но validation сейчас не отклоняет другие непустые значения.

## Проблема

Ожидаемое поведение: неизвестный режим client authentication завершает построение TLS config понятной ошибкой конфигурации.

Фактическое поведение: неизвестная строка проходит `validateServerOptions`; switch затем выбирает `tls.NoClientCert`. При заданном `ClientCA` конфигурация успешно строится, но клиентский сертификат не запрашивается и не проверяется.

Практический риск: опечатка вроде `require-and-verify` молча отключает ожидаемую mTLS-аутентификацию.

Текущий источник: `sdk/tlsconfig/tlsconfig.go`, функции `validateServerOptions`, `effectiveClientAuth` и `NewServer`.

## Решение

- `validateServerOptions` строго разрешает только пустое значение, `none`, `verify_if_given` и `require_and_verify`.
- Пустое значение сохраняет текущую семантику и эквивалентно `none`.
- Любое другое значение возвращает ошибку, содержащую недопустимый mode.
- Существующая проверка обязательного `ClientCA` для двух verifying modes сохраняется.
- Mapping корректных modes в `crypto/tls.ClientAuthType` не меняется.

Публичные типы и константы не меняются. Ранее принятые неизвестные значения намеренно становятся ошибкой fail-closed.

## Проверка

- Regression test строит `ServerOptions` с валидными cert, key и `ClientCA`, но с `ClientAuthMode("require-and-verify")`; до исправления этот набор успешно строит config с `tls.NoClientCert`, после исправления обязан вернуть ошибку.
- Тест не должен получать ложноположительную ошибку из-за отсутствующего CA или невалидной PEM-пары.
- Существующие тесты подтверждают, что пустое значение/`none`, `verify_if_given` и `require_and_verify` продолжают работать и дают ожидаемый `tls.ClientAuthType`.

## Вне scope

- Изменение TLS minimum version, certificate loading или proto schema.
- Нормализация произвольных aliases и исправление опечаток за пользователя.
- Новые mTLS modes.

## Критерии приёмки

- Неизвестный непустой `ClientAuthMode` всегда отклоняется до возврата `*tls.Config`.
- Пустое значение и три документированных режима сохраняют текущее поведение.
- Для verifying modes по-прежнему обязателен `ClientCA`.
- Все существующие тесты проекта проходят.
