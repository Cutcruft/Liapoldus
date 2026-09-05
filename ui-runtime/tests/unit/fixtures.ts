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

// --- routes (таблица для Router/edge) -------------------------------------

export const routeDescriptor1: RouteDescriptor = {
  kind: 'route',
  id: 'home',
  path: '/',
  matcher: { kind: 'regex', source: '/^\\/$/', priority: 0 },
  operationId: 'page@home',
};

export const routeDescriptor2: RouteDescriptor = {
  kind: 'route',
  id: 'docs',
  path: '/docs',
  matcher: { kind: 'regex', source: '/^\\/docs(\\/.*)?$/', priority: 1 },
  operationId: 'page@docs',
};

export const routeDescriptor3: RouteDescriptor = {
  kind: 'route',
  id: 'blog-entry',
  path: '/blog/{slug}/',
  matcher: {
    kind: 'path',
    source: '/blog/',
    priority: 1,
    segments: [
      { type: 'fixed', value: 'blog' },
      { type: 'param', value: 'slug' },
    ],
  },
  operationId: 'page@blog-entry',
};

export const routeDescriptor4: RouteDescriptor = {
  kind: 'route',
  id: 'api',
  path: '/*',
  matcher: { kind: 'regex', source: '/^\\/api\\/.*$/', priority: 100 },
  operationId: 'page@api',
};

export const routeDescriptor5: RouteDescriptor = {
  kind: 'route',
  id: 'not-found',
  path: '/*',
  matcher: { kind: 'path', source: '/*', priority: 0 },
  operationId: 'page@not-found',
};

export const routeDescriptor6: RouteDescriptor = {
  kind: 'route',
  id: 'catalog',
  path: '/catalog',
  matcher: { kind: 'path', source: '/catalog', priority: 10 },
  operationId: 'page@catalog',
};

export const routeDescriptor7: RouteDescriptor = {
  kind: 'route',
  id: 'catalog-old',
  path: '/old-catalog',
  matcher: { kind: 'path', source: '/old-catalog', priority: 10 },
  operationId: 'page@catalog-old',
};

// --- themes --------------------------------------------------------------

export const themeDescriptor1: ThemeDescriptor = {
  kind: 'theme',
  id: 'default',
  tokens: {
    '--color-primary': '#305EA8',
    '--color-bg': '#FFFFFF',
    '--font-body': 'Roboto, sans-serif',
  },
};

export const themeDescriptor2: ThemeDescriptor = {
  kind: 'theme',
  id: 'dark',
  tokens: {
    '--color-primary': '#7EA8E8',
    '--color-bg': '#0F1115',
    '--font-body': 'Inter, sans-serif',
  },
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
    id: 'root',
    definitionId: 'Page',
    props: {
      type: 'children',
      children: [
        {
          id: 'i1',
          definitionId: 'Header',
          props: {
            type: 'children',
            children: [
              {
                id: 'i2',
                definitionId: 'Navigation',
                props: {
                  type: 'children',
                  children: [
                    {
                      id: 'i3',
                      definitionId: 'Item',
                      props: {
                        type: 'props',
                        value: {
                          text: { key: 'text', ref: { source: 'i18n', entityId: 'menu', path: 'home' } },
                          link: '#home',
                        },
                      },
                    },
                    {
                      id: 'i4',
                      definitionId: 'Item',
                      props: {
                        type: 'props',
                        value: {
                          text: { key: 'text', ref: { source: 'i18n', entityId: 'menu', path: 'favorites' } },
                          link: '#favorites',
                        },
                      },
                    },
                  ],
                },
              },
            ],
          },
        },
        {
          id: 'i5',
          definitionId: 'Hero',
          props: {
            type: 'props',
            value: {
              title: { key: 'title', ref: { source: 'content', entityId: 'hero', path: 'strings.title' } },
              body: { key: 'body', ref: { source: 'content', entityId: 'hero', path: 'strings.body' } },
            },
          },
        },
        {
          id: 'i6',
          definitionId: 'ProductList',
          props: {
            type: 'list',
            list: {
              source: { source: 'content', entityId: 'products', path: 'items' },
              as: 'item',
              children: [
                {
                  id: 'i7',
                  definitionId: 'ProductCard',
                  props: {
                    type: 'props',
                    value: { title: { key: 'title', ref: { source: 'content', path: 'title' } } },
                  },
                },
              ],
            },
          },
        },
        {
          id: 'i8',
          definitionId: 'Footer',
          props: {
            type: 'props',
            value: {
              copyright: { key: 'copyright', ref: { source: 'i18n', entityId: 'footer', path: 'copyright' } },
            },
          },
        },
        {
          id: 'i9',
          definitionId: 'NotFoundInfo',
          props: {
            type: 'props',
            value: { text: { key: 'text', ref: { source: 'i18n', entityId: 'errors', path: 'notFound' } } },
          },
        },
        {
          id: 'i10',
          definitionId: 'AuthStatus',
          props: {
            type: 'props',
            value: { hint: { key: 'hint', ref: { source: 'form', entityId: 'contact', path: 'status' } } },
          },
        },
        {
          id: 'i11',
          definitionId: 'Reviews',
          props: {
            type: 'props',
            value: {
              items: { key: 'items', ref: { source: 'operation', entityId: 'reviews', path: 'items.[].title' } },
            },
          },
        },
        {
          id: 'i12',
          definitionId: 'Breadcrumbs',
          props: {
            type: 'props',
            value: { current: { key: 'current', ref: { source: 'route', path: 'params.category' } } },
          },
        },
      ],
    },
  },
};