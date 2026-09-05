import { DescriptorValidationError } from '../errors';
import type { ResolvedOperation } from './registry';
import { RuntimeRegistry } from './registry';
import { PollScheduler, type SchedulerEnv } from './poll-scheduler';
import type { Transport, TransportRequest } from './transport/transport';

export type Channel = 'poll' | 'sse' | 'ws';

export interface SubscriptionMeta {
  timestamp: number;
  channel: Channel;
}

export interface SyncOptions {
  /** cron-выражение, если нужен poll; при отсутствии — берётся poll.schedule из дескриптора */
  schedule?: string;
  /** выполнить первый тик сразу (default true) */
  immediate?: boolean;
  /** принудительный канал (poll/sse/ws) */
  channel?: Channel;
  input?: Record<string, unknown>;
  /** params запроса (query/path/body) — приоритет над input для построения URL */
  params?: Record<string, unknown>;
  errorHandler?: (e: Error) => void;
}

export interface SyncEngineOptions {
  schedulerEnv?: Partial<SchedulerEnv>;
}

/** Отделяет данные (поллятся) от структуры (только rebuild). */
export class SyncEngine {
  private readonly scheduler: PollScheduler;
  private counter = 0;

  constructor(
    private readonly registry: RuntimeRegistry,
    private readonly transport: Transport,
    opts?: SyncEngineOptions,
  ) {
    this.scheduler = new PollScheduler(opts?.schedulerEnv);
  }

  /** Short polling по operationId: каждый тик — отдельный HTTP-запрос. Возвращает отписку. */
  poll<T = unknown>(operationId: string, onData: (d: T, meta: SubscriptionMeta) => void, opts?: SyncOptions): () => void {
    const op = this.requireOperation(operationId);
    const schedule = op.poll?.schedule ?? opts?.schedule;
    if (!schedule) {
      throw new DescriptorValidationError(`Операция '${op.id}' не имеет poll.schedule и не передан opts.schedule`);
    }

    const meta: SubscriptionMeta = { timestamp: Date.now(), channel: 'poll' };
    const self = this;
    const id = `poll-${++self.counter}`;

    const tick = (): Promise<void> =>
      self.executePoll(op, opts, (data) => onData(data as T, { ...meta, timestamp: Date.now() }));

    const job = this.scheduler.add(schedule, tick, {
      immediate: opts?.immediate ?? true,
      errorHandler: opts?.errorHandler,
      id,
    });

    return () => job.cancel();
  }

  /** Подписка по каналу: opts.channel → иначе push (ws/sse), если есть; иначе poll. */
  subscribe<T = unknown>(operationId: string, onData: (d: T, meta: SubscriptionMeta) => void, opts?: SyncOptions): () => void {
    const op = this.requireOperation(operationId);
    const channel = this.pickChannel(op, opts?.channel);
    const meta: SubscriptionMeta = { timestamp: Date.now(), channel };

    if (channel === 'poll') {
      return this.poll(operationId, onData, opts);
    }

    // sse/ws push: поток данных через transport.subscribe
    if (typeof this.transport.subscribe !== 'function') {
      throw new DescriptorValidationError(`Транспорт провайдера '${op.provider.id}' не поддерживает push-канал ${channel}`);
    }
    const req: TransportRequest = { provider: op.provider, operation: op, input: opts?.input, params: opts?.params };
    return this.transport.subscribe(
      req,
      (data) => onData(data as T, { ...meta, timestamp: Date.now() }),
      (e) => {
        if (opts?.errorHandler) opts.errorHandler(e);
      },
    );
  }

  private requireOperation(operationId: string): ResolvedOperation {
    const op = this.registry.findOperationByIdOrBadge(operationId) ?? this.registry.getOperation(operationId);
    if (!op) throw new DescriptorValidationError(`Неизвестная операция '${operationId}'`);
    return op;
  }

  private pickChannel(op: ResolvedOperation, forced?: Channel): Channel {
    if (forced === 'poll') return 'poll';
    if (forced === 'sse' || forced === 'ws') {
      if (!op.subscribe) throw new DescriptorValidationError(`Операция '${op.id}' не может работать по каналу ${forced}: нет subscribe`);
      return forced;
    }
    // push объявлен и у провайдера/операции есть subscribe → push; иначе poll
    const isPush = op.push === true || op.provider.subscribe !== undefined;
    if (isPush && op.subscribe) {
      return op.provider.protocol === 'sse' ? 'sse' : 'ws';
    }
    return 'poll';
  }

  private async executePoll(op: ResolvedOperation, opts: SyncOptions | undefined, emit: (data: unknown) => void): Promise<void> {
    const res = await this.transport.request({
      provider: op.provider,
      operation: op,
      input: opts?.input,
      params: opts?.params,
    });
    emit(res.body);
  }

  get activeSize(): number {
    return this.scheduler.size();
  }
}