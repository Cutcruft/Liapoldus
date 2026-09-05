import { DescriptorValidationError, TransportError } from '../errors';
import type { OperationDescriptor } from '../types/descriptor';
import type { TreeDeclaration } from '../types/tree';
import { ApiClient } from './api-client';
import { AssetResolver } from './assets';
import { registerBuiltin } from './builtin/descriptors';
import { parseCron } from './cron';
import { parseDescriptors } from './descriptor';
import { FormRuntime } from './form';
import { I18n, type I18nOptions } from './i18n';
import { RuntimeRegistry } from './registry';
import { Router } from './router';
import { createRuntimeStore, type RuntimeStore } from './store';
import { SyncEngine } from './sync';
import { DesignTokens } from './tokens';
import { CompoundTransport } from './transport/compound';
import { TransportFactory } from './transport/factory';
import type { FetchLike, WebSocketLike } from './transport/transport';
import { TreeController } from './tree';

/** Инжектируемое окружение boot (тесты: фабрики + storage). */
export interface BootEnv {
  fetch?: FetchLike;
  storage?: { getItem(key: string): string | null; setItem(key: string, value: string): void };
  navigatorLanguage?: string;
  WebSocket?: new (url: string, protocols?: string | string[]) => WebSocketLike;
  /** полный переход при serveAsset */
  window?: { location: { assign(url: string): void } };
}

export interface BootOptions {
  /** base URL контракта (default 'http://localhost') */
  baseUrl?: string;
  /** версия контракта: запрашивается как `versionId` */
  versionId?: string;
  /** переопределение/дополнение настроек локализации (defaultLocale всегда из контракта) */
  i18n?: Omit<I18nOptions, 'defaultLocale'>;
  env?: BootEnv;
  /** обработчик ошибок poll-тиков (для метрик/логов) */
  pollErrorHandler?: (e: Error) => void;
}

/** Результат boot: все сервисы одной строкой (§16#6). */
export interface BootRuntime {
  readonly siteId: string;
  readonly environment: string;
  readonly ready: boolean;
  readonly registry: RuntimeRegistry;
  readonly store: RuntimeStore;
  readonly router: Router;
  readonly sync: SyncEngine;
  readonly client: ApiClient;
  readonly i18n: I18n;
  readonly tokens: DesignTokens;
  readonly tree: TreeController;
  readonly forms: FormRuntime;
  readonly assets: AssetResolver;
  /** полный сброс: закрыть dev-канал, снять poll, выгрузить транспорты и сессию */
  dispose(): void;
}

const DEFAULT_BASE_URL = 'http://localhost';
const CONTRACT_PATH = '/runtime/contract';
const DEV_CHANNEL = 'dev';

/** Сессии рантаймов по siteId: повторный boot перезагружает ту же сессию. */
const sessions = new Map<string, BootSession>();

class BootSession {
  readonly siteId: string;
  readonly registry = new RuntimeRegistry();
  readonly store = createRuntimeStore();
  environment: string;
  opts: BootOptions;

  i18n!: I18n;
  tokens!: DesignTokens;
  router!: Router;
  tree!: TreeController;
  sync!: SyncEngine;
  client!: ApiClient;
  forms!: FormRuntime;
  assets!: AssetResolver;

  ready = false;
  private factory!: TransportFactory;
  private transport!: CompoundTransport;
  private dev?: WebSocketLike;
  private unsubs: Array<() => void> = [];

  constructor(siteId: string, environment: string, opts: BootOptions) {
    this.siteId = siteId;
    this.environment = environment;
    this.opts = opts;
  }

  private get baseUrl(): string {
    return this.opts.baseUrl ?? DEFAULT_BASE_URL;
  }

  async start(): Promise<void> {
    if (!this.factory) {
      this.factory = new TransportFactory({
        fetch: this.opts.env?.fetch,
        WebSocket: this.opts.env?.WebSocket,
      });
      this.transport = new CompoundTransport(this.factory);
    }
    registerBuiltin(this.registry);

    const res = await this.fetchContract();
    const text = await res.text();
    const parsed = parseDescriptors(text);
    const rawTree = this.extractTree(text);

    for (const p of parsed.providers) {
      if (!this.registry.hasProvider(p.id)) this.registry.registerProvider(p);
    }
    for (const op of parsed.operations) {
      if (op.poll?.schedule) this.validateCron(op);
      if (!this.registry.hasOperation(op.id)) this.registry.registerOperation(op);
    }
    for (const ep of parsed.endpoints) {
      if (!this.registry.hasEndpoint(ep.id)) this.registry.registerEndpoint(ep);
    }

    this.i18n = new I18n(
      { defaultLocale: parsed.contract.locale, ...this.opts.i18n },
      this.store,
      { storage: this.opts.env?.storage, navigatorLanguage: this.opts.env?.navigatorLanguage },
    );
    this.tokens = new DesignTokens({ scope: ':root' });
    this.router = new Router(this.store, { window: this.opts.env?.window });
    this.sync = new SyncEngine(this.registry, this.transport);
    this.client = new ApiClient(this.registry, { transport: this.transport, scope: 'server', sync: this.sync });
    this.tree = new TreeController(this.store);
    this.forms = new FormRuntime(this.store, {
      query: (operationId: string, input?: Record<string, unknown>) => this.client.query(operationId, input),
      callEndpoint: (endpointId: string, input?: Record<string, unknown>) => this.client.callEndpoint(endpointId, input),
    }, {
      localeProvider: () => this.i18n.getLocale(),
    });
    this.assets = new AssetResolver(this.store, this.client.builtin.asset);

    this.i18n.detect();
    this.ready = true;

    if (parsed.themes.length > 0) {
      this.tokens.apply(parsed.themes[0]);
      this.store.getState().applyTokens(this.tokens.values());
    }
    this.router.replaceRoutes(parsed.routes);
    this.store.getState().setRoutes(parsed.routes);

    if (rawTree) this.tree.load(rawTree);

    this.startPolls(parsed.operations);
    this.startDevChannel(parsed.contract.capabilities.dev);
  }

  runtime(): BootRuntime {
    return {
      siteId: this.siteId,
      environment: this.environment,
      ready: this.ready,
      registry: this.registry,
      store: this.store,
      router: this.router,
      sync: this.sync,
      client: this.client,
      i18n: this.i18n,
      tokens: this.tokens,
      tree: this.tree,
      forms: this.forms,
      assets: this.assets,
      dispose: () => this.dispose(),
    };
  }

  reset(environment: string, opts: BootOptions): void {
    // #11 повторный boot: Registry.drop + сброс scheduler + полная перерегистрация
    this.environment = environment;
    this.opts = opts;
    this.registry.drop();
    this.sync?.dispose();
    this.closeDev();
    for (const unsub of this.unsubs) unsub();
    this.unsubs = [];
    this.ready = false;
  }

  dispose(): void {
    for (const unsub of this.unsubs) unsub();
    this.unsubs = [];
    this.closeDev();
    this.sync.dispose();
    this.factory.clear();
    sessions.delete(this.siteId);
  }

  private closeDev(): void {
    this.dev?.close();
    this.dev = undefined;
  }

  private async fetchContract(): Promise<Response> {
    const params = new URLSearchParams({ siteId: this.siteId, environment: this.environment });
    if (this.opts.versionId) params.set('versionId', this.opts.versionId);
    const fetchLike = this.opts.env?.fetch;
    if (typeof fetchLike !== 'function') {
      throw new TransportError('boot требует fetch (глобальный или инжектированный в env.fetch)');
    }
    const res = await fetchLike(`${this.baseUrl}${CONTRACT_PATH}?${params.toString()}`);
    if (!res.ok) {
      throw new TransportError(`Контракт сайта '${this.siteId}' недоступен: HTTP ${res.status}`, {
        status: res.status,
      });
    }
    return res;
  }

  private validateCron(op: OperationDescriptor): void {
    try {
      parseCron(op.poll!.schedule);
    } catch {
      throw new DescriptorValidationError(
        `Невалидный poll.schedule '${op.poll!.schedule}' операции '${op.id}'`,
        { entityId: op.id, path: 'poll.schedule' },
      );
    }
  }

  private extractTree(text: string): TreeDeclaration | null {
    let raw: Record<string, unknown>;
    try {
      raw = JSON.parse(text) as Record<string, unknown>;
    } catch {
      raw = {};
    }
    const tree = raw.tree;
    if (tree === null || typeof tree !== 'object' || Array.isArray(tree)) return null;
    return tree as TreeDeclaration;
  }

  private startPolls(ops: OperationDescriptor[]): void {
    for (const op of ops) {
      const schedule = op.poll?.schedule;
      if (!schedule) continue;
      const opId = op.id;
      const push = this.sync.subscribe(
        opId,
        (data) => {
          this.store.getState().setOperationResult(opId, { data });
        },
        {
          input: { siteId: this.siteId, locale: this.i18n.getLocale() },
          params: { siteId: this.siteId, locale: this.i18n.getLocale() },
          errorHandler: this.opts.pollErrorHandler,
        },
      );
      this.unsubs.push(push);
    }
  }

  private startDevChannel(capDev: boolean): void {
    if (this.environment !== 'development' || !capDev) return;
    const WS = this.opts.env?.WebSocket;
    if (!WS) return;
    const ws = new WS(`${this.baseUrl}/runtime/dev`, DEV_CHANNEL);
    ws.addEventListener('message', (event) => {
      const data = (event as { data?: string }).data;
      if (!data) return;
      let msg: { type?: string; tree?: TreeDeclaration };
      try {
        msg = JSON.parse(data) as { type?: string; tree?: TreeDeclaration };
      } catch {
        return;
      }
      if (msg.type === 'tree' && msg.tree) this.tree.rebuild(msg.tree);
    });
    this.dev = ws;
  }
}

/**
 * Запуск runtime (спека §16). Извлекает контракт по GET /runtime/contract (siteId/environment/versionId),
 * регистрирует дескрипторы, ставит тему/роуты/дерево/локаль и стартует poll.
 * Повторный boot того же siteId: Registry.drop(), сброс scheduler, полная перерегистрация (#11).
 */
export async function boot(
  siteId: string,
  environment: string = 'production',
  opts: BootOptions = {},
): Promise<BootRuntime> {
  const existing = sessions.get(siteId);
  if (existing) {
    existing.reset(environment, opts);
  }
  const session = existing ?? new BootSession(siteId, environment, opts);
  await session.start();
  sessions.set(siteId, session);
  return session.runtime();
}