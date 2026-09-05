export type ProviderProtocol = 'http' | 'ws' | 'sse' | 'graphql';

export type OperationType = 'query' | 'mutation';

export type OperationScope = 'public' | 'server';

export type CachePolicy = 'immutable' | 'disabled' | 'ttl';

export interface PollSchedule {
  /** 6-полевый cron: second minute hour dom month dow */
  schedule: string;
  /** интервал в мс (не используется при заданном schedule) */
  intervalMs?: number;
  /** короткое кэширование между опросами (default true) */
  cache?: boolean;
}

export interface ProviderDescriptor {
  kind: 'provider';
  id: string;
  protocol: ProviderProtocol;
  /** отсутствует → встроенные пути (/api, /runtime) того же origin */
  baseUrl?: string;
  defaults?: {
    headers?: Record<string, string>;
    timeoutMs?: number;
  };
  subscribe?: {
    url?: string;
    eventName?: string;
  };
}

export interface SubscribeFields {
  url?: string;
  eventName?: string;
}

export interface OperationDescriptor {
  kind: 'operation';
  id: string;
  typeOp: OperationType;
  providerId: string;
  method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE';
  path?: string;
  params?: {
    in: 'query' | 'path' | 'body';
    fields?: Record<string, { required?: boolean }>;
  };
  input?: { schemaId: string };
  output?: { schemaId: string };
  /** id схемы → тип. Возможные значения см. OperationTypeBinding */
  type?: string;
  cache: CachePolicy;
  ttl?: number;
  scope?: OperationScope;
  poll?: PollSchedule;
  push?: boolean;
  /** WS/SSE: подписка; по умолчанию — из provider.subscribe */
  subscribe?: SubscribeFields;
  headers?: Record<string, string>;
}

/** Привязка `type` в OperationDescriptor к типу результата. */
export type OperationTypeBinding =
  | 'content'
  | 'content[]'
  | 'asset'
  | 'asset[]'
  | 'assets'
  | 'form'
  | 'tree'
  | 'tokens'
  | 'routes[]'
  | 'string'
  | 'raw';

export interface EndpointDescriptor {
  kind: 'endpoint';
  id: string;
  path: string;
  method: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE';
  operationId: string;
  input?: { schemaId: string };
  output?: { schemaId: string };
}

export type RedirectStatus = 301 | 302 | 307 | 308;

export type RouteAction =
  | { type: 'renderPage'; pageId: string }
  | { type: 'serveAsset'; assetId: string }
  | { type: 'redirect'; target: string; status?: RedirectStatus; keepQuery?: boolean };

export interface RouteDescriptor {
  /** id маршрута, напр. `route.article` */
  id: string;
  /** полный regex с обязательными якорями `^…$` (одна функция матчинга для клиента и edge) */
  matcher: string;
  /** больше = раньше; при равенстве — порядок регистрации */
  priority: number;
  action: RouteAction;
}

export interface ResolvedRoute {
  route: RouteDescriptor;
  /** захваченные именованные группы regex */
  params: Record<string, string>;
  /** query-параметры запроса */
  query: Record<string, string>;
}

export type ThemeTokenDef = string | { value: string } | { ref: string };

export interface ThemeDescriptor {
  themeId: string;
  tokens: Record<string, ThemeTokenDef>;
  /** подключаемые шрифты */
  fonts?: string[];
  /** статические ресурсы темы */
  assets?: string[];
}

export interface FallbackDescriptor {
  type: 'fallback-template';
  id: string;
  definition?: string;
  params?: Record<string, string>;
}

export interface ContractDescriptor {
  siteId: string;
  environment: string;
  version: string;
  locale: string;
  protocols: ProviderDescriptor[];
  operations: OperationDescriptor[];
  endpoints: EndpointDescriptor[];
  routes: RouteDescriptor[];
  themes: ThemeDescriptor[];
  fallback?: {
    id: string;
    definition?: string;
    params?: Record<string, string>;
  };
  enabledChannels: {
    ws: boolean;
    sse: boolean;
  };
  capabilities: {
    formSubmissions: boolean;
    dev: boolean;
  };
}

/** `type` беджится в конце id: `op#query`, `endpoint#server`, `provider#ws`, `route`, `theme`. */
export type Descriptor =
  | ProviderDescriptor
  | OperationDescriptor
  | EndpointDescriptor
  | RouteDescriptorKind
  | ThemeDescriptorKind;

/** внутренний дискриминант для register(): валидатор добавляет `kind`. */
export interface RouteDescriptorKind extends RouteDescriptor {
  kind: 'route';
}

export interface ThemeDescriptorKind extends ThemeDescriptor {
  kind: 'theme';
}