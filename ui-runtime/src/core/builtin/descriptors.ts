import type { EndpointDescriptor, OperationDescriptor, ProviderDescriptor } from '../../types/descriptor';
import type { RuntimeRegistry } from '../registry';

/** Провайдер `liapoldus.builtin`: встроенные пути `/api` и `/runtime` того же origin. */
export const BUILTIN_PROVIDER_ID = 'liapoldus.builtin';

export const builtinProviderDescriptor: ProviderDescriptor = {
  kind: 'provider',
  id: BUILTIN_PROVIDER_ID,
  protocol: 'http',
};

/** Типизированные операции к нашему backend (спека §5 «builtin»). */
export const builtinOperationDescriptors: OperationDescriptor[] = [
  {
    kind: 'operation',
    id: 'content.get',
    typeOp: 'query',
    providerId: BUILTIN_PROVIDER_ID,
    method: 'GET',
    path: '/api/contents/{contentId}',
    params: { in: 'query', fields: { contentId: { required: true }, locale: { required: false } } },
    cache: 'immutable',
    type: 'content',
  },
  {
    kind: 'operation',
    id: 'content.list',
    typeOp: 'query',
    providerId: BUILTIN_PROVIDER_ID,
    method: 'GET',
    path: '/api/sites/{siteId}/contents',
    params: {
      in: 'query',
      fields: { siteId: { required: true }, collectionId: { required: false }, locale: { required: false } },
    },
    cache: 'ttl',
    ttl: 120,
    type: 'content[]',
  },
  {
    kind: 'operation',
    id: 'content.batch',
    typeOp: 'query',
    providerId: BUILTIN_PROVIDER_ID,
    method: 'POST',
    path: '/api/sites/{siteId}/contents/batch',
    params: { in: 'body' },
    cache: 'immutable',
    type: 'content[]',
  },
  {
    kind: 'operation',
    id: 'asset.get',
    typeOp: 'query',
    providerId: BUILTIN_PROVIDER_ID,
    method: 'GET',
    path: '/api/assets/{assetId}',
    params: { in: 'query', fields: { assetId: { required: true } } },
    cache: 'immutable',
    type: 'asset',
  },
  {
    kind: 'operation',
    id: 'asset.list',
    typeOp: 'query',
    providerId: BUILTIN_PROVIDER_ID,
    method: 'GET',
    path: '/api/sites/{siteId}/assets',
    params: { in: 'query', fields: { siteId: { required: true } } },
    cache: 'immutable',
    type: 'asset[]',
  },
  {
    kind: 'operation',
    id: 'asset.batch',
    typeOp: 'query',
    providerId: BUILTIN_PROVIDER_ID,
    method: 'POST',
    path: '/api/sites/{siteId}/assets/batch',
    params: { in: 'body' },
    cache: 'immutable',
    type: 'assets',
  },
  {
    kind: 'operation',
    id: 'form.get',
    typeOp: 'query',
    providerId: BUILTIN_PROVIDER_ID,
    method: 'GET',
    path: '/api/forms/{formId}',
    params: { in: 'query', fields: { formId: { required: true } } },
    cache: 'ttl',
    ttl: 300,
    type: 'form',
  },
  {
    kind: 'operation',
    id: 'form.submit',
    typeOp: 'mutation',
    providerId: BUILTIN_PROVIDER_ID,
    method: 'POST',
    path: '/api/forms/{formId}/submissions',
    params: { in: 'path', fields: { formId: { required: true } } },
    cache: 'disabled',
    scope: 'server',
  },
  {
    kind: 'operation',
    id: 'tree.get',
    typeOp: 'query',
    providerId: BUILTIN_PROVIDER_ID,
    method: 'GET',
    path: '/runtime/tree',
    params: { in: 'query', fields: { routeId: { required: false }, locale: { required: false } } },
    cache: 'immutable',
    type: 'tree',
  },
  {
    kind: 'operation',
    id: 'tokens.get',
    typeOp: 'query',
    providerId: BUILTIN_PROVIDER_ID,
    method: 'GET',
    path: '/runtime/tokens',
    params: { in: 'query', fields: { themeId: { required: false } } },
    cache: 'immutable',
    type: 'tokens',
  },
  {
    kind: 'operation',
    id: 'routes.get',
    typeOp: 'query',
    providerId: BUILTIN_PROVIDER_ID,
    method: 'GET',
    path: '/runtime/routes',
    params: { in: 'query' },
    cache: 'immutable',
    type: 'routes[]',
  },
];

export const builtinEndpointDescriptors: EndpointDescriptor[] = [
  {
    kind: 'endpoint',
    id: 'form.submit',
    path: '/api/forms/{formId}/submissions',
    method: 'POST',
    operationId: 'form.submit',
  },
];

export interface RegisterBuiltinOptions {
  /** переопределить id провайдера (для тестов) */
  providerId?: string;
}

/** Регистрирует builtin-провайдер/операции/эндпоинты (при boot). Идемпотентна: существующие регистрации не перезаписываются. */
export function registerBuiltin(registry: RuntimeRegistry, opts?: RegisterBuiltinOptions): void {
  const providerId = opts?.providerId ?? BUILTIN_PROVIDER_ID;
  if (!registry.hasProvider(providerId)) {
    registry.registerProvider({ ...builtinProviderDescriptor, id: providerId });
  }
  const ops = opts?.providerId
    ? builtinOperationDescriptors.map((op) => (op.providerId === BUILTIN_PROVIDER_ID ? { ...op, providerId: opts.providerId! } : op))
    : builtinOperationDescriptors;
  for (const op of ops) {
    if (!registry.hasOperation(op.id)) registry.registerOperation(op);
  }
  for (const ep of builtinEndpointDescriptors) {
    if (!registry.hasEndpoint(ep.id)) registry.registerEndpoint(ep);
  }
}