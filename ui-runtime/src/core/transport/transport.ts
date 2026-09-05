import type { ResolvedOperation, ResolvedProvider } from '../registry';

export interface TransportRequest {
  provider: ResolvedProvider;
  operation: ResolvedOperation;
  /** параметры операции (query/path/body) */
  params?: Record<string, unknown>;
  /** тело/входные данные */
  input?: Record<string, unknown>;
  /** кастомные headers (приоритет над defaults/operation) */
  headers?: Record<string, string>;
  /** переопределение таймаута */
  timeoutMs?: number;
  /** GraphQL: строка запроса */
  gql?: string;
  /** логический аргумент подписки (для ws/sse) */
  subscribe?: boolean;
}

export interface TransportResponse {
  status: number;
  ok: boolean;
  body: unknown;
}

export interface Transport {
  request(req: TransportRequest): Promise<TransportResponse>;
  /** push-канал (ws/sse): поток данных, возвращённая функция отписывает */
  subscribe?(req: TransportRequest, onData: (data: unknown) => void, onError?: (e: Error) => void): () => void;
  close?: () => void;
}

export type FetchLike = (
  input: string | URL | RequestInfo,
  init?: RequestInit,
) => Promise<Response>;

export interface FrameLike {
  data?: string;
  event?: string;
}

export interface WebSocketLike {
  url?: string;
  addEventListener: (type: string, cb: (event: unknown) => void) => void;
  removeEventListener?: (type: string, cb: (event: unknown) => void) => void;
  send: (data: string) => void;
  close: () => void;
}

export interface WebSocketCtor {
  new (url: string, protocols?: string | string[]): WebSocketLike;
}

export interface EventSourceLike {
  url: string;
  addEventListener: (type: string, cb: (event: unknown) => void) => void;
  removeEventListener?: (type: string, cb: (event: unknown) => void) => void;
  close: () => void;
}

export interface EventSourceCtor {
  new (url: string): EventSourceLike;
}

/** Инжектируемое окружение транспортов (фейки в тестах). */
export interface TransportEnv {
  fetch?: FetchLike;
  WebSocket?: WebSocketCtor | null;
  EventSource?: EventSourceCtor | null;
}

export function pickFetch(env?: TransportEnv): FetchLike {
  const fn = env?.fetch ?? globalThis.fetch;
  if (typeof fn !== 'function') {
    throw new Error('Транспорт требует fetch (глобальный или инжектированный)');
  }
  return fn.bind(globalThis);
}

export function pickWebSocket(env?: TransportEnv): WebSocketCtor {
  const ctor = env?.WebSocket ?? (globalThis as { WebSocket?: WebSocketCtor }).WebSocket;
  if (typeof ctor !== 'function') {
    throw new Error('Транспорт требует WebSocket');
  }
  return ctor;
}

export function pickEventSource(env?: TransportEnv): EventSourceCtor {
  const ctor = env?.EventSource ?? (globalThis as { EventSource?: EventSourceCtor }).EventSource;
  if (typeof ctor !== 'function') {
    throw new Error('Транспорт требует EventSource');
  }
  return ctor;
}