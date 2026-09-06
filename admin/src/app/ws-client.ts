import { DEFAULT_BUILDS_WS_PATH, DEFAULT_DEV_WS_PATH } from '../runtime';
import type { BuildEvent, DevRebuildEvent } from '../runtime';

/** Приёмник WS-события (интерфейс, совместимый с browser WebSocket). */
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

/**
 * Подписка на live-WS-канал с переподключением по экспоненциальной задержке.
 * Единый общий клиент для dev-ребортера и событий сборок.
 */
export function connectWs<T>(
  opts: {
    path: string;
    siteId: string;
    token?: string;
    onEvent: (event: T) => void;
  },
  createSocket: SocketFactory = defaultSocket,
): () => void {
  const { path, siteId, token, onEvent } = opts;
  if (typeof WebSocket === 'undefined' && createSocket === defaultSocket) return () => {};
  if (siteId === '') return () => {};

  let closed = false;
  let retries = 0;
  let ws: WsLike | undefined;
  let timer: ReturnType<typeof setTimeout> | undefined;

  const protocol = typeof location !== 'undefined' && location.protocol === 'https:' ? 'wss:' : 'ws:';
  const host = typeof location !== 'undefined' ? location.host : 'localhost:5173';
  const tokenPart = token ? `&token=${encodeURIComponent(token)}` : '';
  const url = `${protocol}//${host}${path}?siteId=${encodeURIComponent(siteId)}${tokenPart}`;

  const open = (): void => {
    if (closed) return;
    ws = createSocket(url);
    ws.onmessage = (event) => {
      try {
        onEvent(JSON.parse(event.data) as T);
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

// --- Тонкие типизированные обёртки (совместимость с прежними consumer'ами) ---

/** Переопределяемые фабрики сокетов (авто-тесты подменяют на фейк). */
let devFactory: SocketFactory | undefined;
let buildFactory: SocketFactory | undefined;

export function setDevSocketFactory(factory?: SocketFactory): void {
  devFactory = factory;
}

export function setBuildSocketFactory(factory?: SocketFactory): void {
  buildFactory = factory;
}

export function connectDevWs(
  siteId: string,
  onEvent: (event: DevRebuildEvent) => void,
  createSocket: SocketFactory = devFactory ?? defaultSocket,
): () => void {
  return connectWs({ path: DEFAULT_DEV_WS_PATH, siteId, onEvent }, createSocket);
}

export function connectBuildWs(
  siteId: string,
  token: string,
  onEvent: (event: BuildEvent) => void,
  createSocket: SocketFactory = buildFactory ?? defaultSocket,
): () => void {
  return connectWs({ path: DEFAULT_BUILDS_WS_PATH, siteId, token, onEvent }, createSocket);
}
