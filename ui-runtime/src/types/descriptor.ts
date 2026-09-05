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

export type MatcherKind = 'path' | 'regex';

export interface RouteDescriptor {
  kind: 'route';
  id: string;
  /** путь маршрута для матчинга в клиенте, например `/*` */
  path: string;
  matcher: {
    kind: MatcherKind;
    source: string;
    priority?: number;
    segments?: { type: 'fixed' | 'param'; value: string }[];
  };
  operationId: string;
}

export interface ThemeToken {
  [name: string]: string | ThemeToken;
}

export interface ThemeDescriptor {
  kind: 'theme';
  id: string;
  tokens: ThemeToken;
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
  | RouteDescriptor
  | ThemeDescriptor;