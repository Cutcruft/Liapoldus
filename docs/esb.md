# ESB layer для расширений

ESB — отдельный универсальный gRPC transport для расширений. Он не заменяет типизированный `ManagementService`.

```text
ManagementService
  → typed protobuf requests
  → управление Liapoldus

Esb
  → opaque bytes + metadata
  → plugin/provider extensions
```

## Контракт

Контракт находится в [proto/esb/esb.proto](../proto/esb/esb.proto). Он является внешним и не должен изменяться без согласования с владельцем шины.

Маршрутизация выполняется по metadata:

```text
service       — логическое имя расширения
method        — имя операции
content_type  — формат payload
correlation_id — идентификатор корреляции
```

`payload` остаётся непрозрачным массивом байт. Его формат может быть JSON, Protobuf, BSON, MessagePack или другим форматом, который понимает конкретное расширение.

## Registry

Расширение регистрирует operation в [backend/esb/registry.go](../backend/esb/registry.go):

```go
registry.Register("catalog", "Search", esb.Operation{
    Call: func(ctx context.Context, request esb.Request) (esb.Reply, error) {
        // decode request.Payload according to request.Metadata["content_type"]
        return esb.Reply{Payload: response}, nil
    },
})
```

У одной operation может быть `Call`, `Stream` или оба обработчика.

## Call

`Call` используется для request/response:

```text
Esb.Call
  → metadata service/method
  → Registry
  → extension handler
  → EsbReply
```

Если во входящем запросе есть `correlation_id`, registry автоматически возвращает его в metadata ответа, если обработчик не задал другое значение.

## Stream

`Stream` используется для последовательных результатов, событий и долгоживущих операций. Обработчик отправляет ответы через callback. Отмена gRPC context останавливает обработку.

## Ограничения

По умолчанию размер payload ограничен 4 MiB. Ограничение задаётся при создании registry. Обязательны metadata `service`, `method` и `content_type`.

Ошибки преобразуются в gRPC codes:

```text
invalid request / malformed metadata → InvalidArgument
unknown operation                  → NotFound
payload over limit                 → ResourceExhausted
cancelled / deadline               → Canceled / DeadlineExceeded
```

## Запуск

В режиме gRPC сервер публикует оба сервиса на одном listener:

```bash
LIAPOLDUS_MANAGEMENT_TRANSPORT=grpc \
LIAPOLDUS_ADDR=:9090 \
go run ./cmd/server
```

В режиме REST запускается только REST management transport. ESB является gRPC-слоем и в REST-режиме не активируется.

Для проверки подключенного ESB в WSL можно выполнить:

```bash
wsl.exe -d Ubuntu -- bash -lc "cd /mnt/c/Users/sh_ub/CutCraft/Liapoldus && GRPCURL_BIN=/home/sh_ub/go/bin/grpcurl bash scripts/test-esb.sh"
```

Тест отправляет запрос к незарегистрированной операции и проверяет, что сервер возвращает `NotFound`. Реальные операции появляются после регистрации расширения в registry.
