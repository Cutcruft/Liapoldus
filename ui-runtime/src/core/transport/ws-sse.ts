import { SubscriptionError, TransportError } from '../../errors';
import type { ResolvedOperation } from '../registry';
import type { Transport, TransportRequest, TransportResponse, WebSocketCtor, WebSocketLike } from './transport';

/** URL подключения WS-канала: op.subscribe → provider.subscribe → baseUrl+path. */
export function wsSubscribeUrl(operation: ResolvedOperation): string | undefined {
  const opUrl = operation.subscribe?.url;
  if (opUrl) return opUrl;
  const provUrl = operation.provider.subscribe?.url;
  if (provUrl) return provUrl;
  if (operation.provider.baseUrl) {
    return `${operation.provider.baseUrl.replace(/\/$/, '')}${operation.path ?? ''}`;
  }
  return undefined;
}

export class WebSocketTransport implements Transport {
  private socket: WebSocketLike | null = null;

  constructor(
    _provider: { id: string },
    private readonly ctor: WebSocketCtor,
  ) {}

  async request(req: TransportRequest): Promise<TransportResponse> {
    const url = wsSubscribeUrl(req.operation);
    if (!url) {
      throw new TransportError(`Нет URL для WebSocket-канала операции '${req.operation.id}'`);
    }
    const timeoutMs = req.timeoutMs ?? 30_000;
    return new Promise<TransportResponse>((resolve, reject) => {
      let settled = false;
      const ws = new this.ctor(url);
      this.socket = ws;

      const cleanup = (): void => {
        try {
          ws.close();
        } catch {
          /* noop */
        }
        if (timer) clearTimeout(timer);
      };

      const timer = setTimeout(() => {
        if (!settled) {
          settled = true;
          cleanup();
          reject(new TransportError(`Таймаут ожидания сообщения от '${req.operation.id}'`));
        }
      }, timeoutMs);

      const parseFrame = (data: unknown): unknown => {
        if (typeof data !== 'string') return data;
        try {
          return JSON.parse(data);
        } catch {
          return data;
        }
      };

      ws.addEventListener('message', (event) => {
        if (settled) return;
        settled = true;
        const frame = (event as FrameEventLike).data;
        cleanup();
        resolve({ status: 200, ok: true, body: parseFrame(frame) });
      });

      ws.addEventListener('error', (event) => {
        if (settled) return;
        settled = true;
        cleanup();
        reject(new TransportError(`WebSocket-ошибка для '${req.operation.id}'`, { message: (event as { message?: string })?.message }));
      });

      ws.addEventListener('open', () => {
        // канал открыт; ждём первое сообщение
      });

      ws.addEventListener('close', (event) => {
        if (settled) return;
        settled = true;
        cleanup();
        reject(new TransportError(`WebSocket закрыт для '${req.operation.id}' (${(event as { code?: number })?.code ?? 'unknown'})`));
      });
    });
  }

  close(): void {
    if (this.socket) {
      try {
        this.socket.close();
      } catch {
        /* noop */
      }
      this.socket = null;
    }
  }

  /** Push-подписка: каждое сообщение → onData; возвращённая функция отписывает. */
  subscribe(req: TransportRequest, onData: (data: unknown) => void, onError?: (e: Error) => void): () => void {
    const url = wsSubscribeUrl(req.operation);
    if (!url) {
      const err = new TransportError(`Нет URL для WebSocket-канала операции '${req.operation.id}'`);
      if (onError) onError(err);
      return () => undefined;
    }
    const ws = new this.ctor(url);
    this.socket = ws;

    const parseFrame = (data: unknown): unknown => {
      if (typeof data !== 'string') return data;
      try {
        return JSON.parse(data);
      } catch {
        return data;
      }
    };

    ws.addEventListener('message', (event) => {
      const frame = (event as FrameEventLike).data;
      onData(parseFrame(frame));
    });
    ws.addEventListener('error', (event) => {
      onError?.(new TransportError(`WebSocket-ошибка для '${req.operation.id}'`, { message: (event as { message?: string })?.message }));
    });
    ws.addEventListener('close', () => {
      onError?.(new TransportError(`WebSocket-канал закрыт для '${req.operation.id}'`));
    });

    let closed = false;
    return () => {
      if (closed) return;
      closed = true;
      try {
        ws.close();
      } catch {
        /* noop */
      }
      if (this.socket === ws) this.socket = null;
    };
  }
}

export interface FrameEventLike {
  data: string;
}

/** SSE-транспорт: подписка через EventSource. */
export class SseTransport implements Transport {
  private source: { close: () => void } | null = null;

  constructor(private readonly ctor: { new (url: string): EventSourceLike0 }) {}

  async request(req: TransportRequest): Promise<TransportResponse> {
    const url = wsSubscribeUrl(req.operation);
    if (!url) throw new SubscriptionError(`SSE-канал '${req.operation.id}' не имеет url`);
    const eventName = req.operation.subscribe?.eventName ?? req.operation.provider.subscribe?.eventName ?? 'message';
    const timeoutMs = req.timeoutMs ?? 30_000;

    const body = await new Promise<unknown>((resolve, reject) => {
      let settled = false;
      const source = new this.ctor(url);
      this.source = source;
      const cleanup = (): void => {
        try {
          source.close();
        } catch {
          /* noop */
        }
        if (timer) clearTimeout(timer);
      };
      const timer = setTimeout(() => {
        if (!settled) {
          settled = true;
          cleanup();
          reject(new SubscriptionError(`Таймаут SSE-подписки '${req.operation.id}'`));
        }
      }, timeoutMs);

      const onMessage = (event: unknown): void => {
        if (settled) return;
        settled = true;
        cleanup();
        const data = (event as { data?: string }).data;
        resolve(parseFrame(data));
      };
      const onError = (): void => {
        if (settled) return;
        settled = true;
        cleanup();
        reject(new SubscriptionError(`SSE-ошибка соединения '${req.operation.id}'`));
      };
      source.addEventListener(eventName, onMessage);
      source.addEventListener('error', onError);
    });
    return { status: 200, ok: true, body };
  }

  close(): void {
    if (this.source) {
      try {
        this.source.close();
      } catch {
        /* noop */
      }
      this.source = null;
    }
  }

  /** Push-подписка: каждое событие → onData; возвращённая функция отписывает. */
  subscribe(req: TransportRequest, onData: (data: unknown) => void, onError?: (e: Error) => void): () => void {
    const url = wsSubscribeUrl(req.operation);
    if (!url) {
      onError?.(new SubscriptionError(`SSE-канал '${req.operation.id}' не имеет url`));
      return () => undefined;
    }
    const eventName = req.operation.subscribe?.eventName ?? req.operation.provider.subscribe?.eventName ?? 'message';
    const source = new this.ctor(url);
    this.source = source;

    const onMessage = (event: unknown): void => {
      onData(parseFrame((event as { data?: string }).data));
    };
    const onErr = (): void => {
      onError?.(new SubscriptionError(`SSE-ошибка соединения '${req.operation.id}'`));
    };
    source.addEventListener(eventName, onMessage);
    source.addEventListener('error', onErr);

    let closed = false;
    return () => {
      if (closed) return;
      closed = true;
      try {
        source.close();
      } catch {
        /* noop */
      }
      if (this.source === source) this.source = null;
    };
  }
}

interface EventSourceLike0 {
  url?: string;
  addEventListener: (type: string, cb: (event: unknown) => void) => void;
  close: () => void;
}

function parseFrame(data: unknown): unknown {
  if (typeof data !== 'string') return data;
  try {
    return JSON.parse(data);
  } catch {
    return data;
  }
}