import {
  DescriptorValidationError,
} from '../errors';
import type {
  ContractDescriptor,
  EndpointDescriptor,
  OperationDescriptor,
  ProviderDescriptor,
  RouteAction,
  RouteDescriptor,
  RouteDescriptorKind,
  ThemeDescriptor,
  ThemeDescriptorKind,
  ThemeTokenDef,
} from '../types/descriptor';

const BADGE_RE = /^([A-Za-z0-9._/-]+)#([a-z]+)$/;

export function stripComments(json: string): string {
  return json
    .replace(/\/\*[\s\S]*?\*\//g, '')
    .replace(/(^|[^\\:])\/\/[^\n\r]*/g, '$1')
    .replace(/,(\s*[}\]])/g, '$1');
}

/** Разбирает JSON дескриптора, толерантно к комментариям и trailing-запятым (§2). */
export function parseContractJSON(json: string): unknown {
  try {
    return JSON.parse(stripComments(json));
  } catch (err) {
    throw new DescriptorValidationError(
      `Ошибка JSON: ${err instanceof Error ? err.message : String(err)}`,
    );
  }
}

function isRecord(v: unknown): v is Record<string, unknown> {
  return typeof v === 'object' && v !== null && !Array.isArray(v);
}

function requireString(obj: Record<string, unknown>, field: string, entityId?: string): string {
  const v = obj[field];
  if (typeof v !== 'string' || v.length === 0) {
    throw new DescriptorValidationError(`Поле '${field}' должно быть непустой строкой`, { entityId, path: field });
  }
  return v;
}

/** приводит id вида `name#query` к чистой строке и возвращает бейдж парсинга */
export function splitBadgedId(id: string): { name: string; badge?: string } {
  const m = BADGE_RE.exec(id);
  if (m) return { name: m[1], badge: m[2] };
  return { name: id };
}

export function validateProviderDescriptor(raw: unknown): ProviderDescriptor {
  if (!isRecord(raw)) throw new DescriptorValidationError('Дескриптор provider должен быть объектом');
  const id = requireString(raw, 'id', 'provider');
  const protocol = raw.protocol;
  if (protocol !== 'http' && protocol !== 'ws' && protocol !== 'sse' && protocol !== 'graphql') {
    throw new DescriptorValidationError(`Протокол '${String(protocol)}' не поддерживается`, { entityId: id, path: 'protocol' });
  }
  const baseUrlValue = raw.baseUrl;
  if (baseUrlValue !== undefined && typeof baseUrlValue !== 'string') {
    throw new DescriptorValidationError('baseUrl должен быть строкой', { entityId: id, path: 'baseUrl' });
  }
  const provider: ProviderDescriptor = {
    kind: 'provider',
    id,
    protocol,
  };
  if (typeof baseUrlValue === 'string') provider.baseUrl = baseUrlValue;
  if (isRecord(raw.defaults)) {
    const defaults: NonNullable<ProviderDescriptor['defaults']> = {};
    if (isRecord(raw.defaults.headers)) defaults.headers = raw.defaults.headers as Record<string, string>;
    if (typeof raw.defaults.timeoutMs === 'number') defaults.timeoutMs = raw.defaults.timeoutMs;
    if (Object.keys(defaults).length > 0) provider.defaults = defaults;
  }
  if (isRecord(raw.subscribe)) {
    const subscribe: NonNullable<ProviderDescriptor['subscribe']> = {};
    if (typeof raw.subscribe.url === 'string') subscribe.url = raw.subscribe.url;
    if (typeof raw.subscribe.eventName === 'string') subscribe.eventName = raw.subscribe.eventName;
    provider.subscribe = subscribe;
  }
  return provider;
}

const METHODS = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE'];
const OP_TYPES = ['query', 'mutation'];
const SCOPES = ['public', 'server'];
const TYPED_BINDINGS = ['content', 'content[]', 'asset', 'asset[]', 'assets', 'form', 'tree', 'tokens', 'routes[]'];

export function validateOperationDescriptor(raw: unknown): OperationDescriptor {
  if (!isRecord(raw)) throw new DescriptorValidationError('Дескриптор operation должен быть объектом');
  const id = requireString(raw, 'id', 'operation');
  const typeOp = raw.typeOp ?? raw.type_operation;
  if (!OP_TYPES.includes(typeOp as string)) {
    throw new DescriptorValidationError(`type должен быть 'query' или 'mutation'`, { entityId: id, path: 'typeOp' });
  }
  requireString(raw, 'providerId', id);
  const method = raw.method ?? 'GET';
  if (!METHODS.includes(method as string)) {
    throw new DescriptorValidationError(`method '${String(method)}' не поддерживается`, { entityId: id, path: 'method' });
  }
  const cache = raw.cache;
  if (cache !== 'immutable' && cache !== 'disabled' && cache !== 'ttl') {
    throw new DescriptorValidationError('cache должен быть immutable|disabled|ttl', { entityId: id, path: 'cache' });
  }
  const scope = raw.scope ?? 'public';
  if (!SCOPES.includes(scope as string)) {
    throw new DescriptorValidationError('scope должен быть public|server', { entityId: id, path: 'scope' });
  }
  const op: OperationDescriptor = {
    kind: 'operation',
    id,
    typeOp: typeOp as OperationDescriptor['typeOp'],
    providerId: raw.providerId as string,
    method: method as OperationDescriptor['method'],
    cache: cache as OperationDescriptor['cache'],
    scope: scope as OperationDescriptor['scope'],
  };
  if (typeof raw.path === 'string') op.path = raw.path;
  if (typeof raw.ttl === 'number') op.ttl = raw.ttl;
  if (raw.push === true) op.push = true;
  if (isRecord(raw.params)) {
    const pin = raw.params.in;
    if (pin !== 'query' && pin !== 'path' && pin !== 'body') {
      throw new DescriptorValidationError('params.in должен быть query|path|body', { entityId: id, path: 'params.in' });
    }
    const params: OperationDescriptor['params'] = { in: pin as 'query' | 'path' | 'body' };
    if (isRecord(raw.params.fields)) {
      params.fields = {};
      for (const [field, spec] of Object.entries(raw.params.fields as Record<string, unknown>)) {
        params.fields[field] = { required: isRecord(spec) && spec.required === true };
      }
    }
    op.params = params;
  }
  if (isRecord(raw.input) && typeof raw.input.schemaId === 'string') op.input = { schemaId: raw.input.schemaId };
  if (isRecord(raw.output) && typeof raw.output.schemaId === 'string') op.output = { schemaId: raw.output.schemaId };
  if (typeof raw.type === 'string') {
    if (!TYPED_BINDINGS.includes(raw.type) && raw.type !== 'string' && raw.type !== 'raw') {
      throw new DescriptorValidationError(`type '${raw.type}' не является известной привязкой`, { entityId: id, path: 'type' });
    }
    op.type = raw.type;
  }
  if (typeof raw.poll === 'object' && raw.poll !== null) {
    const poll = raw.poll as Record<string, unknown>;
    if (typeof poll.schedule !== 'string' && typeof poll.intervalMs !== 'number') {
      throw new DescriptorValidationError('poll требует schedule (cron) или intervalMs', { entityId: id, path: 'poll' });
    }
    op.poll = { schedule: typeof poll.schedule === 'string' ? poll.schedule : '' };
    if (typeof poll.intervalMs === 'number') op.poll.intervalMs = poll.intervalMs;
    if (typeof poll.cache === 'boolean') op.poll.cache = poll.cache;
  }
  if (isRecord(raw.subscribe)) {
    const subscribe: NonNullable<OperationDescriptor['subscribe']> = {};
    if (typeof raw.subscribe.url === 'string') subscribe.url = raw.subscribe.url;
    if (typeof raw.subscribe.eventName === 'string') subscribe.eventName = raw.subscribe.eventName;
    op.subscribe = subscribe;
  }
  if (isRecord(raw.headers)) op.headers = raw.headers as Record<string, string>;
  return op;
}

export function validateEndpointDescriptor(raw: unknown): EndpointDescriptor {
  if (!isRecord(raw)) throw new DescriptorValidationError('Дескриптор endpoint должен быть объектом');
  const id = requireString(raw, 'id', 'endpoint');
  requireString(raw, 'operationId', id);
  const method = raw.method ?? 'POST';
  if (!METHODS.includes(method as string)) {
    throw new DescriptorValidationError(`method '${String(method)}' не поддерживается`, { entityId: id, path: 'method' });
  }
  const ep: EndpointDescriptor = {
    kind: 'endpoint',
    id,
    path: typeof raw.path === 'string' ? raw.path : '/',
    method: method as EndpointDescriptor['method'],
    operationId: raw.operationId as string,
  };
  if (isRecord(raw.input) && typeof raw.input.schemaId === 'string') ep.input = { schemaId: raw.input.schemaId };
  if (isRecord(raw.output) && typeof raw.output.schemaId === 'string') ep.output = { schemaId: raw.output.schemaId };
  return ep;
}

export function validateRouteDescriptor(raw: unknown): RouteDescriptorKind {
  if (!isRecord(raw)) throw new DescriptorValidationError('Дескриптор route должен быть объектом');
  const id = requireString(raw, 'id', 'route');
  const matcher = requireString(raw, 'matcher', id);
  if (!matcher.startsWith('^') || !matcher.endsWith('$')) {
    throw new DescriptorValidationError(
      `matcher маршрута '${id}' должен быть полным regex с якорями ^…$`,
      { entityId: id, path: 'matcher' },
    );
  }
  new RegExp(matcher); // ранняя проверка компиляции
  const priority = typeof raw.priority === 'number' ? raw.priority : 0;
  if (!isRecord(raw.action)) {
    throw new DescriptorValidationError('route требует action', { entityId: id, path: 'action' });
  }
  const type = raw.action.type;
  if (type === 'renderPage') {
    requireString(raw.action, 'pageId', id);
  } else if (type === 'serveAsset') {
    requireString(raw.action, 'assetId', id);
  } else if (type === 'redirect') {
    requireString(raw.action, 'target', id);
    const status = raw.action.status;
    if (status !== undefined && (status !== 301 && status !== 302 && status !== 307 && status !== 308)) {
      throw new DescriptorValidationError(
        `status редиректа маршрута '${id}' вне {301,302,307,308}`,
        { entityId: id, path: 'action.status' },
      );
    }
  } else {
    throw new DescriptorValidationError(`action.type маршрута '${id}' должен быть renderPage|serveAsset|redirect`, {
      entityId: id,
      path: 'action.type',
    });
  }
  return { kind: 'route', id, matcher, priority, action: raw.action as RouteAction };
}

export function validateThemeDescriptor(raw: unknown): ThemeDescriptorKind {
  if (!isRecord(raw)) throw new DescriptorValidationError('Дескриптор theme должен быть объектом');
  const themeId = requireString(raw, 'themeId', 'theme');
  const tokens = raw.tokens;
  if (!isRecord(tokens)) {
    throw new DescriptorValidationError('theme требует tokens', { entityId: themeId, path: 'tokens' });
  }
  for (const [name, def] of Object.entries(tokens)) {
    if (typeof def === 'string') continue;
    if (isRecord(def) && typeof def.value === 'string') continue;
    if (isRecord(def) && typeof def.ref === 'string') continue;
    throw new DescriptorValidationError(
      `Токен '${name}' темы '${themeId}' должен быть string | { value } | { ref }`,
      { entityId: themeId, path: `tokens.${name}` },
    );
  }
  const theme: ThemeDescriptorKind = {
    kind: 'theme',
    themeId,
    tokens: { ...(tokens as Record<string, ThemeTokenDef>) },
  };
  if (Array.isArray(raw.fonts)) theme.fonts = raw.fonts.filter((f): f is string => typeof f === 'string');
  if (Array.isArray(raw.assets)) theme.assets = raw.assets.filter((a): a is string => typeof a === 'string');
  return theme;
}

export interface ParseResult {
  contract: ContractDescriptor;
  providers: ProviderDescriptor[];
  operations: OperationDescriptor[];
  endpoints: EndpointDescriptor[];
  routes: RouteDescriptor[];
  themes: ThemeDescriptor[];
}

/** Парсит и валидирует весь контракт. Бросает DescriptorValidationError. */
export function parseDescriptors(json: string): ParseResult {
  const raw = parseContractJSON(json);
  if (!isRecord(raw)) throw new DescriptorValidationError('Корень контракта должен быть объектом');

  const list = (key: 'providers' | 'operations' | 'endpoints' | 'routes' | 'themes'): unknown[] => {
    const arr = raw[key];
    if (arr === undefined) return [];
    if (!Array.isArray(arr)) throw new DescriptorValidationError(`'${key}' должен быть массивом`, { path: key });
    return arr;
  };

  const providers = list('providers').map(validateProviderDescriptor);
  const operations = list('operations').map(validateOperationDescriptor);
  const endpoints = list('endpoints').map(validateEndpointDescriptor);
  const routes = list('routes').map(validateRouteDescriptor);
  const themes = list('themes').map(validateThemeDescriptor);

  const enabledChannels = { ws: true, sse: true };
  if (isRecord(raw.enabledChannels)) {
    if (typeof raw.enabledChannels.ws === 'boolean') enabledChannels.ws = raw.enabledChannels.ws;
    if (typeof raw.enabledChannels.sse === 'boolean') enabledChannels.sse = raw.enabledChannels.sse;
  }
  const capabilities = { formSubmissions: true, dev: false };
  if (isRecord(raw.capabilities)) {
    if (typeof raw.capabilities.formSubmissions === 'boolean') capabilities.formSubmissions = raw.capabilities.formSubmissions;
    if (typeof raw.capabilities.dev === 'boolean') capabilities.dev = raw.capabilities.dev;
  }

  const contract: ContractDescriptor = {
    siteId: typeof raw.siteId === 'string' ? raw.siteId : 'unknown',
    environment: typeof raw.environment === 'string' ? raw.environment : 'prod',
    version: typeof raw.version === 'string' ? raw.version : '0',
    locale: typeof raw.locale === 'string' ? raw.locale : 'ru',
    protocols: providers,
    operations,
    endpoints,
    routes,
    themes,
    enabledChannels,
    capabilities,
  };
  if (isRecord(raw.fallback)) {
    contract.fallback = {
      id: typeof raw.fallback.id === 'string' ? raw.fallback.id : '',
      ...(typeof raw.fallback.definition === 'string' ? { definition: raw.fallback.definition } : {}),
      ...(isRecord(raw.fallback.params) ? { params: raw.fallback.params as Record<string, string> } : {}),
    };
  }

  return { contract, providers, operations, endpoints, routes, themes };
}