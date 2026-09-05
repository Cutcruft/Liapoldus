import { CronParseError } from '../errors';
import { nextTick, parseCron } from './cron';

export interface SchedulerEnv {
  setTimeout: typeof setTimeout;
  clearTimeout: typeof clearTimeout;
  now: () => Date;
}

type TimeoutToken = ReturnType<typeof setTimeout>;

interface SchedulerImpl {
  setTimeout: (fn: () => void, ms: number) => TimeoutToken;
  clearTimeout: (t: TimeoutToken) => void;
  now: () => Date;
}

export interface PollJob {
  readonly id: string;
  cancel(): void;
}

interface PollJobInternal {
  readonly id: string;
  delay: () => number;
  tick: () => void | Promise<void>;
  errorHandler?: (e: Error) => void;
  token?: TimeoutToken;
  running: boolean;
  dead: boolean;
  cancel: () => void;
}

/**
 * Планировщик переборов по cron (6 полей) или интервалу (мс).
 * Каждый тик планирует следующий запуск без дрейфа; перекрытие долгого тика игнорируется.
 */
export class PollScheduler {
  private readonly jobs = new Map<string, PollJobInternal>();
  private readonly impl: SchedulerImpl;
  private counter = 0;

  constructor(env?: Partial<SchedulerEnv>) {
    this.impl = {
      setTimeout: env?.setTimeout ?? ((fn, ms) => setTimeout(fn, ms)),
      clearTimeout: env?.clearTimeout ?? ((t) => clearTimeout(t)),
      now: env?.now ?? (() => new Date()),
    };
  }

  add(
    schedule: string | number,
    tick: () => void | Promise<void>,
    opts?: { immediate?: boolean; errorHandler?: (e: Error) => void; id?: string },
  ): PollJob {
    let cronExpr: ReturnType<typeof parseCron> | undefined;
    if (typeof schedule === 'string') {
      cronExpr = parseCron(schedule);
    } else if (typeof schedule !== 'number') {
      throw new CronParseError('Расписание должно быть cron-строкой или числом (мс)');
    }

    const id = opts?.id ?? `job-${++this.counter}`;

    const delay = (): number => {
      if (cronExpr) {
        return Math.max(1, nextTick(this.impl.now(), cronExpr).getTime() - this.impl.now().getTime());
      }
      return Math.max(1, Math.trunc(schedule as number));
    };

    const internal: PollJobInternal = {
      id,
      delay,
      tick,
      errorHandler: opts?.errorHandler,
      running: false,
      dead: false,
      cancel: () => this.remove(id),
    };
    this.jobs.set(id, internal);

    const arm = (): void => {
      if (internal.dead) return;
      const ms = internal.delay();
      let token: TimeoutToken;
      token = this.impl.setTimeout(() => this.fire(id, internal, token), ms);
      internal.token = token;
    };

    if (opts?.immediate === false) {
      arm();
    } else {
      void this.run(internal).then(() => {
        if (!internal.dead) arm();
      });
    }
    return internal;
  }

  private run(job: PollJobInternal): Promise<void> {
    job.running = true;
    let p: Promise<unknown>;
    try {
      p = Promise.resolve(job.tick());
    } catch (e) {
      p = Promise.reject(e);
    }
    return p
      .catch((e) => {
        if (job.errorHandler) job.errorHandler(e as Error);
      })
      .then(() => {
        job.running = false;
      });
  }

  private fire(id: string, job: PollJobInternal, token: TimeoutToken): void {
    if (job.dead || job.token !== token) return;
    job.token = undefined;
    if (job.running) return;
    void this.run(job).then(() => {
      if (!job.dead && !job.running) {
        const ms = job.delay();
        let t2: TimeoutToken;
        t2 = this.impl.setTimeout(() => this.fire(id, job, t2), ms);
        job.token = t2;
      }
    });
  }

  private remove(id: string): void {
    const job = this.jobs.get(id);
    if (!job) return;
    job.dead = true;
    if (job.token) this.impl.clearTimeout(job.token);
    this.jobs.delete(id);
  }

  size(): number {
    return this.jobs.size;
  }

  has(id: string): boolean {
    const job = this.jobs.get(id);
    return !!job && !job.dead;
  }

  clear(): void {
    for (const id of Array.from(this.jobs.keys())) this.remove(id);
  }
}