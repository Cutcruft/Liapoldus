import type { FetchLike } from '../../src/core/transport/transport';

export interface FetchCall {
  url: string;
  init: RequestInit;
}

export interface FakeResponse {
  ok: boolean;
  status: number;
  body: unknown;
  headers?: Record<string, string>;
}

/** Фейковый fetch: записывает вызовы, отдаёт handler-результат как Response. */
export function makeFakeFetch(
  handler: (call: FetchCall) => unknown | Error | Promise<unknown> | Promise<Error>,
): { fetch: FetchLike; calls: FetchCall[] } {
  const calls: FetchCall[] = [];
  const fetchImpl: FetchLike = async (input, init) => {
    const call: FetchCall = { url: String(input), init: init ?? {} };
    calls.push(call);
    const result = await handler(call);
    if (result instanceof Error) throw result;
    if (result && typeof result === 'object' && 'status' in (result as object)) {
      return result as unknown as Response;
    }
    const body = result as unknown;
    return {
      ok: true,
      status: 200,
      headers: new Headers(),
      async json() {
        return body;
      },
      async text() {
        return typeof body === 'string' ? body : JSON.stringify(body);
      },
    } as unknown as Response;
  };
  return { fetch: fetchImpl, calls };
}

export function fakeFetchNever(): FetchLike {
  return () => new Promise<Response>(() => undefined);
}

export function fakeResponseJson(response: FakeResponse): FakeResponse {
  return response;
}

export interface FakeWsOpts {
  url: string;
}

/** Фейковый WebSocket (инжектируется в транспорты). */
export class FakeWebSocket {
  static instances: FakeWebSocket[] = [];
  url: string;
  closed = false;
  private listeners = new Map<string, Array<(event: unknown) => void>>();

  constructor(url: string) {
    this.url = url;
    FakeWebSocket.instances.push(this);
  }

  addEventListener(type: string, cb: (event: unknown) => void): void {
    const arr = this.listeners.get(type) ?? [];
    arr.push(cb);
    this.listeners.set(type, arr);
  }

  removeEventListener(type: string, cb: (event: unknown) => void): void {
    const arr = this.listeners.get(type) ?? [];
    this.listeners.set(type, arr.filter((f) => f !== cb));
  }

  send(): void {
    /* noop */
  }

  close(): void {
    this.closed = true;
    this.dispatch('close', { code: 1000 });
  }

  dispatch(type: string, event: unknown): void {
    for (const cb of this.listeners.get(type) ?? []) cb(event);
  }

  emitOpen(): void {
    this.dispatch('open', {});
  }

  emitMessage(data: string): void {
    this.dispatch('message', { data });
  }

  emitError(message?: string): void {
    this.dispatch('error', { message });
  }
}

/** Фейковый EventSource для SSE-транспорта. */
export class FakeEventSource {
  static instances: FakeEventSource[] = [];
  url: string;
  closed = false;
  private listeners = new Map<string, Array<(event: unknown) => void>>();

  constructor(url: string) {
    this.url = url;
    FakeEventSource.instances.push(this);
  }

  addEventListener(type: string, cb: (event: unknown) => void): void {
    const arr = this.listeners.get(type) ?? [];
    arr.push(cb);
    this.listeners.set(type, arr);
  }

  removeEventListener(type: string, cb: (event: unknown) => void): void {
    const arr = this.listeners.get(type) ?? [];
    this.listeners.set(type, arr.filter((f) => f !== cb));
  }

  close(): void {
    this.closed = true;
  }

  dispatch(type: string, event: unknown): void {
    for (const cb of this.listeners.get(type) ?? []) cb(event);
  }

  emitMessage(data: string): void {
    this.dispatch('update', { data });
  }

  emitError(): void {
    this.dispatch('error', {});
  }
}

export function resetFakes(): void {
  FakeWebSocket.instances = [];
  FakeEventSource.instances = [];
}