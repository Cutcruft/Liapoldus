import type { DevRebuildEvent } from '../../runtime';

/** Приёмник события dev-ребортера (интерфейс, совместимый с browser WebSocket). */
export interface WsLike {
  onmessage: ((event: { data: string }) => void) | null;
  onopen: (() => void) | null;
  onclose: (() => void) | null;
  onerror: (() => void) | null;
  close: () => void;
}

export type SocketFactory = (url: string) => WsLike;

const START_DELAY_MS = 1000;
const MAX_DELAY_MS = 10_000;

function defaultSocket(url: string): WsLike {
  return new WebSocket(url) as unknown as WsLike;
}

/** Переопределяемая фабрика сокетов (авто-тесты подменяют на фейк). */
let activeFactory: SocketFactory | undefined;

export function setDevSocketFactory(factory?: SocketFactory): void {
  activeFactory = factory;
}

/**
 * Подписка на live-события сборки dev сайта (`/dev/build/ws?siteId=`).
 * Переподключается с экспоненциальной задержкой; возвращает cleanup.
 */
export function connectDevWs(
  siteId: string,
  onEvent: (event: DevRebuildEvent) => void,
  createSocket: SocketFactory = activeFactory ?? defaultSocket,
): () => void {
  if (typeof WebSocket === 'undefined' && activeFactory === undefined) return () => {};
  if (siteId === '') return () => {};

  let closed = false;
  let retries = 0;
  let ws: WsLike | undefined;
  let timer: ReturnType<typeof setTimeout> | undefined;

  const protocol = typeof location !== 'undefined' && location.protocol === 'https:' ? 'wss:' : 'ws:';
  const host = typeof location !== 'undefined' ? location.host : 'localhost:5173';
  const url = `${protocol}//${host}/dev/build/ws?siteId=${encodeURIComponent(siteId)}`;

  const open = (): void => {
    if (closed) return;
    ws = createSocket(url);
    ws.onmessage = (event) => {
      try {
        onEvent(JSON.parse(event.data) as DevRebuildEvent);
      } catch {
        // не-JSON событие — игнорируем
      }
    };
    ws.onopen = () => {
      retries = 0;
    };
    ws.onclose = () => {
      if (closed) return;
      const delay = Math.min(START_DELAY_MS * 2 ** retries, MAX_DELAY_MS);
      retries += 1;
      timer = setTimeout(open, delay);
    };
    ws.onerror = () => {
      ws?.close();
    };
  };

  open();

  return () => {
    closed = true;
    if (timer) clearTimeout(timer);
    ws?.close();
  };
}