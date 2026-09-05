import type {
  ContractDescriptor,
  EndpointDescriptor,
  OperationDescriptor,
  ProviderDescriptor,
  RouteDescriptor,
  ThemeDescriptor,
} from '../../src/types/descriptor';
import type { TreeDeclaration } from '../../src/types/tree';

// --- providers -----------------------------------------------------------

export const providerDescriptor1: ProviderDescriptor = {
  kind: 'provider',
  id: 'cms',
  protocol: 'http',
  baseUrl: 'https://cms.example.com/api',
  defaults: { headers: { 'X-App-Version': '2' }, timeoutMs: 40 },
};

export const providerDescriptor2: ProviderDescriptor = {
  kind: 'provider',
  id: 'sse',
  protocol: 'sse',
  baseUrl: 'https://live.example.com',
  subscribe: { url: 'https://live.example.com/events', eventName: 'update' },
};

export const providerDescriptor3: ProviderDescriptor = {
  kind: 'provider',
  id: 'ws',
  protocol: 'ws',
  baseUrl: 'wss://ws.example.com',
  subscribe: { url: 'wss://ws.example.com/events', eventName: 'update' },
};

export const providerDescriptor4: ProviderDescriptor = {
  kind: 'provider',
  id: 'gql',
  protocol: 'graphql',
  baseUrl: 'https://graphql.example.com/graphql',
};

export const providerDescriptor5: ProviderDescriptor = {
  kind: 'provider',
  id: 'noBase',
  protocol: 'http',
};

export const providerDescriptorBuiltin: ProviderDescriptor = {
  kind: 'provider',
  id: 'builtin',
  protocol: 'http',
};

// --- operations ----------------------------------------------------------

export const operationDescriptor1: OperationDescriptor = {
  kind: 'operation',
  id: 'content.get',
  typeOp: 'query',
  providerId: 'cms',
  path: '/contents/{id}',
  method: 'GET',
  params: { in: 'query', fields: { id: { required: true } } },
  input: { schemaId: 'content.get.in' },
  output: { schemaId: 'content' },
  cache: 'immutable',
  type: 'content',
};

export const operationDescriptor2: OperationDescriptor = {
  kind: 'operation',
  id: 'content.list',
  typeOp: 'query',
  providerId: 'cms',
  path: '/contents',
  method: 'GET',
  params: {
    in: 'query',
    fields: { locale: { required: false }, cursor: { required: false }, pageSize: { required: false } },
  },
  cache: 'ttl',
  ttl: 120,
};

export const operationDescriptor3: OperationDescriptor = {
  kind: 'operation',
  id: 'site.register',
  typeOp: 'mutation',
  providerId: 'cms',
  path: '/register',
  method: 'POST',
  params: { in: 'path' },
  cache: 'disabled',
  output: { schemaId: 'visitor' },
};

export const operationDescriptor4: OperationDescriptor = {
  kind: 'operation',
  id: 'content.batch',
  typeOp: 'query',
  providerId: 'cms',
  path: '/contents/batch',
  method: 'POST',
  params: { in: 'body' },
  output: { schemaId: 'content[]' },
  cache: 'immutable',
  type: 'content[]',
};

export const operationDescriptor5: OperationDescriptor = {
  kind: 'operation',
  id: 'content.patch',
  typeOp: 'mutation',
  providerId: 'cms',
  path: '/contents/{id}',
  method: 'PATCH',
  params: { in: 'path', fields: { id: { required: true } } },
  cache: 'disabled',
};

export const operationDescriptor6: OperationDescriptor = {
  kind: 'operation',
  id: 'sse.subscribe',
  typeOp: 'query',
  providerId: 'sse',
  method: 'GET',
  path: '/events',
  cache: 'disabled',
  poll: { schedule: '5 * * * * MON' },
  subscribe: { url: 'https://live.example.com/events', eventName: 'update' },
};

export const operationDescriptor7: OperationDescriptor = {
  kind: 'operation',
  id: 'ws.subscribe',
  typeOp: 'query',
  providerId: 'ws',
  method: 'GET',
  cache: 'disabled',
  push: true,
  subscribe: { url: 'wss://ws.example.com/events', eventName: 'update' },
};

export const operationDescriptor8: OperationDescriptor = {
  kind: 'operation',
  id: 'gql.query',
  typeOp: 'query',
  providerId: 'gql',
  cache: 'disabled',
};

export const operationDescriptor9: OperationDescriptor = {
  kind: 'operation',
  id: 'open.query',
  typeOp: 'query',
  providerId: 'noBase',
  path: '/api/open',
  method: 'GET',
  cache: 'disabled',
};

export const operationDescriptor10: OperationDescriptor = {
  kind: 'operation',
  id: 'form.submit',
  typeOp: 'mutation',
  providerId: 'builtin',
  path: '/api/forms/{formId}/submissions',
  method: 'POST',
  params: { in: 'path', fields: { formId: { required: true } } },
  cache: 'disabled',
  scope: 'server',
};

export const operationDescriptor11: OperationDescriptor = {
  kind: 'operation',
  id: 'content.poll',
  typeOp: 'query',
  providerId: 'cms',
  path: '/poll',
  method: 'GET',
  params: { in: 'query', fields: { locale: { required: false } } },
  cache: 'disabled',
  poll: { schedule: '0 * * * * *' },
};

// --- endpoints -----------------------------------------------------------

export const endpointDescriptor1: EndpointDescriptor = {
  kind: 'endpoint',
  id: 'form.submit',
  path: '/api/forms/{formId}/submissions',
  method: 'POST',
  operationId: 'form.submit',
};

export const endpointDescriptor2: EndpointDescriptor = {
  kind: 'endpoint',
  id: 'contact.submit',
  path: '/api/forms/contact/submissions',
  method: 'POST',
  operationId: 'form.submit',
};

// --- routes (единая таблица для Router/edge, json-descriptors §4) --------

export const routeDescriptor1: RouteDescriptor = {
  id: 'route.home',
  matcher: '^/(|new)$',
  priority: 0,
  action: { type: 'renderPage', pageId: 'page.home' },
};

export const routeDescriptor2: RouteDescriptor = {
  id: 'route.article',
  matcher: '^/articles/(?<articleId>[0-9]+)$',
  priority: 10,
  action: { type: 'renderPage', pageId: 'page.article' },
};

export const routeDescriptor3: RouteDescriptor = {
  id: 'route.blog',
  matcher: '^/blog/(?<slug>[a-z0-9-]+)$',
  priority: 10,
  action: { type: 'renderPage', pageId: 'page.blog' },
};

export const routeDescriptor4: RouteDescriptor = {
  id: 'route.old',
  matcher: '^/old$',
  priority: 5,
  action: { type: 'redirect', target: '/new', status: 301 },
};

export const routeDescriptor5: RouteDescriptor = {
  id: 'route.legacy',
  matcher: '^/legacy/(.*)$',
  priority: 5,
  action: { type: 'redirect', target: '/modern/$1', status: 308, keepQuery: true },
};

export const routeDescriptor6: RouteDescriptor = {
  id: 'route.robots',
  matcher: '^/robots\\.txt$',
  priority: 100,
  action: { type: 'serveAsset', assetId: 'asset.robots' },
};

export const routeDescriptor7: RouteDescriptor = {
  id: 'route.article-lower',
  matcher: '^/articles/(?<articleId>[0-9]+)$',
  priority: 5,
  action: { type: 'renderPage', pageId: 'page.article-lower' },
};

// --- themes (json-descriptors §8) ----------------------------------------

export const themeDescriptor1: ThemeDescriptor = {
  themeId: 'default',
  tokens: {
    '--color-primary': '#305EA8',
    '--color-bg': '#FFFFFF',
    '--font-body': 'Roboto, sans-serif',
  },
};

export const themeDescriptor2: ThemeDescriptor = {
  themeId: 'dark',
  tokens: {
    '--color-primary': '#7EA8E8',
    '--color-bg': '#0F1115',
    '--font-body': 'Inter, sans-serif',
  },
  fonts: ['Inter'],
};

// --- контракт ------------------------------------------------------------

export const contractDescriptor: ContractDescriptor = {
  siteId: 'interactive-content',
  environment: 'prod',
  version: 'v1.0.0',
  locale: 'ru',
  protocols: [
    providerDescriptor1,
    providerDescriptor2,
    providerDescriptor3,
    providerDescriptor4,
    providerDescriptor5,
    providerDescriptorBuiltin,
  ],
  operations: [
    operationDescriptor1,
    operationDescriptor2,
    operationDescriptor3,
    operationDescriptor4,
    operationDescriptor5,
    operationDescriptor6,
    operationDescriptor7,
    operationDescriptor8,
    operationDescriptor9,
    operationDescriptor10,
  ],
  endpoints: [endpointDescriptor1, endpointDescriptor2],
  routes: [routeDescriptor1, routeDescriptor2, routeDescriptor3, routeDescriptor4, routeDescriptor5],
  themes: [themeDescriptor1, themeDescriptor2],
  enabledChannels: { ws: true, sse: true },
  capabilities: { formSubmissions: true, dev: false },
};

export const contractJson = JSON.stringify(
  {
    siteId: 'interactive-content',
    environment: 'prod',
    version: 'v1.0.0',
    locale: 'ru',
    providers: [
      { kind: 'provider', id: 'cms', protocol: 'http', baseUrl: 'https://cms.example.com/api' },
      { kind: 'provider', id: 'sse', protocol: 'sse', baseUrl: 'https://live.example.com' },
      { kind: 'provider', id: 'ws', protocol: 'ws', baseUrl: 'wss://ws.example.com' },
      { kind: 'provider', id: 'gql', protocol: 'graphql', baseUrl: 'https://graphql.example.com/graphql' },
      { kind: 'provider', id: 'noBase', protocol: 'http' },
      { kind: 'provider', id: 'builtin', protocol: 'http' },
    ],
    operations: [
      operationDescriptor1,
      operationDescriptor2,
      operationDescriptor3,
      operationDescriptor4,
      operationDescriptor5,
      operationDescriptor6,
      operationDescriptor7,
      operationDescriptor8,
      operationDescriptor9,
      operationDescriptor10,
    ],
    endpoints: [endpointDescriptor1, endpointDescriptor2],
    routes: [routeDescriptor1, routeDescriptor2, routeDescriptor3, routeDescriptor4, routeDescriptor5],
    themes: [themeDescriptor1, themeDescriptor2],
    fallback: { id: 'custom', definition: 'unmatched' },
    enabledChannels: { ws: true, sse: true },
    capabilities: { formSubmissions: true, dev: false },
  },
  null,
  2,
);

// --- дерево --------------------------------------------------------------

export const treeDeclaration: TreeDeclaration = {
  root: {
    instanceId: 'root',
    definitionId: 'Page',
    props: {},
    bindings: [
      { property: 'title', source: { type: 'content', contentId: 'hero', path: 'strings.title' } },
      { property: 'items', source: { type: 'operation', operationId: 'reviews', path: 'items.[].title' } },
    ],
    children: [
      {
        instanceId: 'i1',
        definitionId: 'Header',
        props: { nav: ['home', 'favorites'] },
        bindings: [],
        children: [
          {
            instanceId: 'i2',
            definitionId: 'Navigation',
            props: {},
            bindings: [{ property: 'label', source: { type: 'content', contentId: 'menu', path: 'home' } }],
            children: [],
          },
        ],
      },
      {
        instanceId: 'i3',
        definitionId: 'Hero',
        props: {},
        bindings: [
          { property: 'title', source: { type: 'content', contentId: 'hero', path: 'strings.title' } },
          { property: 'body', source: { type: 'content', contentId: 'hero', path: 'strings.body' } },
        ],
        children: [],
      },
      {
        instanceId: 'i4',
        definitionId: 'ProductList',
        props: {},
        bindings: [
          { property: 'items', source: { type: 'content', contentId: 'products', path: 'items' } },
          { property: 'title', source: { type: 'content', contentId: 'hero', path: 'strings.title' } },
        ],
        children: [],
      },
      {
        instanceId: 'i5',
        definitionId: 'Footer',
        props: {},
        bindings: [
          { property: 'copyright', source: { type: 'content', contentId: 'footer', path: 'copyright' } },
        ],
        children: [],
      },
    ],
  },
};