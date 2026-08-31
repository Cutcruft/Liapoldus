# Прямые интеграции frontend

## Правило

Если внешний источник может безопасно принимать запрос из браузера, frontend обращается к нему напрямую. Liapoldus не проксирует этот запрос через Go backend.

```text
Component
  → Binding
  → Operation
  → Provider
  → External API
```

Backend отвечает за описание и публикацию интеграции:

- `Operation` и её тип (`Query` или `Mutation`);
- schema входных параметров и результата;
- публичный URL или идентификатор внешнего сервиса;
- разрешённые HTTP-методы и headers без секретов;
- cache policy для frontend;
- версию конфигурации в Snapshot.

Backend не получает и не пересылает внешний payload.

## Provider

Provider является frontend-кодом, поставляемым приложением или Plugin. Он знает протокол внешней системы, но получает настройки из декларативной конфигурации Operation.

Условный контракт:

```ts
interface FrontendProvider {
  execute<TInput, TResult>(
    operation: PublicOperation,
    input: TInput,
    context: RuntimeContext,
  ): Promise<TResult>
}
```

Компонент по-прежнему не знает, используется ли HTTP, GraphQL или другой протокол. Он обращается к Operation через binding/runtime SDK.

## Что можно отдавать в браузер

Публичная конфигурация может содержать URL публичного API, public client ID, имя GraphQL operation, не секретные headers, timeout и frontend cache policy.

Нельзя отдавать API keys, private tokens, credentials к базе данных, внутренние URL, секреты подписи webhook или административные права. Всё, что оказалось в JavaScript, DevTools или сетевом запросе браузера, считается публичным.

## Когда нужен серверный endpoint

Серверный endpoint допускается, если требуется скрыть credential, обратиться к локальной БД или внутренней сети, проверить серверную сессию, принять webhook или выполнить доверенную административную операцию.

Такой endpoint должен быть описан как `server-only operation`, иметь фиксированный contract, allowlist действий и серверную валидацию входа. Он не должен автоматически становиться универсальным прокси.

## CORS и безопасность

Прямой вызов требует настройки CORS на стороне внешнего API. Дополнительно frontend должен использовать CSP, allowlist доменов, ограничение методов и schema validation.

Если внешний сервис не поддерживает CORS или требует секретный ключ, прямой вызов невозможен. Нужно либо настроить внешний сервис, либо реализовать отдельную server-only operation.

## Cache

Cache для прямых операций находится на стороне frontend, браузера, CDN или внешнего API. Backend хранит policy в Operation и публикует её frontend. Серверный cache применяется только к server-only operations.
