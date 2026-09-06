import type { BuildEvent } from '../../runtime';

/** Приёмник build-события (интерфейс, совместимый с browser WebSocket). */
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

export function setBuildSocketFactory(factory?: SocketFactory): void {
  activeFactory = factory;
}

/**
 * Подписка на live-события сборок сайта (`/api/builds/ws?siteId=&token=`).
 * Токен передаётся query-параметром — браузерный WebSocket не умеет в
 * заголовки. Переподключение с экспоненциальной задержкой; cleanup отписывает.
 */
export function connectBuildWs(
  siteId: string,
  token: string,
  onEvent: (event: BuildEvent) => void,
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
  const tokenPart = token ? `&token=${encodeURIComponent(token)}` : '';
  const url = `${protocol}//${host}/api/builds/ws?siteId=${encodeURIComponent(siteId)}${tokenPart}`;

  const open = (): void => {
    if (closed) return;
    ws = createSocket(url);
    ws.onmessage = (event) => {
      try {
        onEvent(JSON.parse(event.data) as BuildEvent);
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