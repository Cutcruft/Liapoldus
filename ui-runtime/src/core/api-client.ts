import {
  OperationKindMismatchError,
  ScopeError,
  UnknownEntityError,
} from '../errors';
import type { OperationScope, OperationType } from '../types/descriptor';
import { BuiltinClient } from './builtin/builtin-client';
import type { ResolvedOperation, RuntimeRegistry } from './registry';
import { SyncEngine, type SubscriptionMeta, type SyncOptions } from './sync';
import type { Transport, TransportRequest } from './transport/transport';

interface CacheEntry {
  value: unknown;
  expiresAt?: number;
}

export interface ApiClientOptions {
  transport: Transport;
  /** контекст вызова: 'public' (компонент) → callEndpoint запрещён, 'server' (edge) → разрешён */
  scope?: OperationScope;
  /** включить кэш query (policy из дескриптора). Default true */
  cache?: boolean;
  /** инжектируемый SyncEngine (для тестов) */
  sync?: SyncEngine;
}

/**
 * Единый API-клиент: query/mutate через реестр, callEndpoint (server-only),
 * typed builtin, poll/subscribe -> SyncEngine.
 */
export class ApiClient {
  readonly builtin: BuiltinClient;
  private readonly scope: OperationScope;
  private readonly cacheEnabled: boolean;
  private readonly cache = new Map<string, CacheEntry>();
  private readonly sync: SyncEngine;

  constructor(
    private readonly registry: RuntimeRegistry,
    private readonly opts: ApiClientOptions,
  ) {
    this.scope = opts.scope ?? 'public';
    this.cacheEnabled = opts.cache ?? true;
    this.sync = opts.sync ?? new SyncEngine(registry, opts.transport);
    this.builtin = new BuiltinClient({
      query: (operationId: string, input?: Record<string, unknown>) => this.query(operationId, input),
      callEndpoint: (endpointId: string, input?: Record<string, unknown>) => this.callEndpoint(endpointId, input),
    });
  }

  async query<TInput extends object | undefined = undefined, TOutput = unknown>(
    operationId: string,
    input?: TInput,
  ): Promise<TOutput> {
    const op = this.resolveOperation('query', operationId);
    return (await this.execute(op, input as Record<string, unknown> | undefined)) as TOutput;
  }

  async mutate<TInput extends object | undefined = undefined, TOutput = unknown>(
    operationId: string,
    input?: TInput,
  ): Promise<TOutput> {
    const op = this.resolveOperation('mutation', operationId);
    return (await this.execute(op, input as Record<string, unknown> | undefined)) as TOutput;
  }

  /** Server-only: вызов серверной операции по эндпоинту. */
  async callEndpoint<TInput extends object | undefined = undefined, TOutput = unknown>(
    endpointId: string,
    input?: TInput,
  ): Promise<TOutput> {
    if (this.scope !== 'server') {
      throw new ScopeError(`Endpoint '${endpointId}' доступен только из server-контекста`);
    }
    const endpoint = this.registry.getEndpoint(endpointId);
    if (!endpoint) {
      throw new UnknownEntityError(`Endpoint '${endpointId}' не найден`);
    }
    return (await this.execute(endpoint.operation, input as Record<string, unknown> | undefined)) as TOutput;
  }

  /** Push-подписка (ws/sse); rollback к poll, если push не объявлен. Возвращает отписку. */
  subscribe<T = unknown>(operationId: string, onData: (d: T, meta: SubscriptionMeta) => void): () => void {
    return this.sync.subscribe(operationId, onData);
  }

  /** Short polling по operationId; opts.schedule = cron, параметры — из input. */
  poll<T = unknown>(operationId: string, onData: (d: T, meta: SubscriptionMeta) => void, opts?: SyncOptions): () => void {
    return this.sync.poll(operationId, onData, { ...opts, params: opts?.input });
  }

  private resolveOperation(typeOp: OperationType, id: string): ResolvedOperation {
    const direct = this.registry.findOperationByIdOrBadge(id) ?? this.registry.getOperation(id);
    if (direct) {
      if (direct.typeOp !== typeOp) {
        throw new OperationKindMismatchError(`Операция '${direct.id}' — '${direct.typeOp}', а требуется '${typeOp}'`);
      }
      return direct;
    }
    const op = this.registry.getOperationFor(typeOp, id);
    if (op) return op;
    throw new UnknownEntityError(`Операция '${id}' не найдена`);
  }

  private async execute(op: ResolvedOperation, input: Record<string, unknown> | undefined): Promise<unknown> {
    const policy = op.cache ?? 'disabled';
    const key = this.cacheEnabled && policy !== 'disabled' ? cacheKey(op.id, input) : undefined;

    if (key !== undefined) {
      const hit = this.cache.get(key);
      if (hit) {
        if (hit.expiresAt === undefined || hit.expiresAt > Date.now()) return hit.value;
        this.cache.delete(key);
      }
    }

    const req: TransportRequest = {
      provider: op.provider,
      operation: op,
      params: { ...(input ?? {}) },
    };
    const res = await this.opts.transport.request(req);

    if (key !== undefined) {
      this.cache.set(key, {
        value: res.body,
        expiresAt: policy === 'ttl' && typeof op.ttl === 'number' ? Date.now() + op.ttl * 1000 : undefined,
      });
    }
    return res.body;
  }
}

function cacheKey(operationId: string, input: Record<string, unknown> | undefined): string {
  return `${operationId}\u0000${JSON.stringify(input ?? {})}`;
}