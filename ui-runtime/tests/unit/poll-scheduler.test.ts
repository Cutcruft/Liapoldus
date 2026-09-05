import { describe, it, expect, vi, afterEach } from 'vitest';
import { PollScheduler } from '../../src/core/poll-scheduler';
import { nextTick, parseCron } from '../../src/core/cron';

const BASE = '2026-09-06T00:00:05.000Z';

function makeEnv() {
  return { setTimeout, clearTimeout, now: () => new Date(BASE) };
}

afterEach(() => {
  vi.useRealTimers();
});

describe('PollScheduler', () => {
  it('immediate (default true): первый тик сразу, затем по cron', async () => {
    vi.useFakeTimers();
    const s = new PollScheduler(makeEnv());
    const tick = vi.fn();
    s.add('* * * * * *', tick);
    expect(tick).toHaveBeenCalledTimes(1);
    expect(s.size()).toBe(1);
    await vi.advanceTimersByTimeAsync(1000);
    expect(tick).toHaveBeenCalledTimes(2);
    await vi.advanceTimersByTimeAsync(1000);
    expect(tick).toHaveBeenCalledTimes(3);
  });

  it('cron: первый отложенный тик ровно по расписанию', async () => {
    vi.useFakeTimers();
    const s = new PollScheduler(makeEnv());
    const tick = vi.fn();
    s.add('5 * * * * *', tick, { immediate: false });
    const first = nextTick(new Date(BASE), parseCron('5 * * * * *')).getTime() - Date.parse(BASE);

    await vi.advanceTimersByTimeAsync(first - 1);
    expect(tick).not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(1);
    expect(tick).toHaveBeenCalledTimes(1);

    await vi.advanceTimersByTimeAsync(first);
    expect(tick).toHaveBeenCalledTimes(2);
    await vi.advanceTimersByTimeAsync(first);
    expect(tick).toHaveBeenCalledTimes(3);
  });

  it('интервал в мс: стабильные тики', async () => {
    vi.useFakeTimers();
    const s = new PollScheduler(makeEnv());
    const tick = vi.fn();
    s.add(100, tick, { immediate: false });
    await vi.advanceTimersByTimeAsync(100);
    expect(tick).toHaveBeenCalledTimes(1);
    await vi.advanceTimersByTimeAsync(100);
    expect(tick).toHaveBeenCalledTimes(2);
  });

  it('cancel: тик больше не происходит', async () => {
    vi.useFakeTimers();
    const s = new PollScheduler(makeEnv());
    const tick = vi.fn();
    const job = s.add(100, tick, { immediate: false });
    job.cancel();
    expect(s.size()).toBe(0);
    await vi.advanceTimersByTimeAsync(500);
    expect(tick).toHaveBeenCalledTimes(0);
  });

  it('clear останавливает все задачи', async () => {
    vi.useFakeTimers();
    const s = new PollScheduler(makeEnv());
    s.add(100, vi.fn());
    s.add(100, vi.fn());
    expect(s.size()).toBe(2);
    s.clear();
    expect(s.size()).toBe(0);
  });

  it('долгий тик не перекрывается', async () => {
    vi.useFakeTimers();
    const s = new PollScheduler(makeEnv());
    const order: number[] = [];
    s.add(
      100,
      () => {
        order.push(1);
        return new Promise<void>((resolve) => {
          setTimeout(resolve, 250);
        });
      },
      { immediate: false },
    );

    await vi.advanceTimersByTimeAsync(100); // t=100: тик 1 стартует (длится до t=350)
    expect(order).toEqual([1]);

    await vi.advanceTimersByTimeAsync(200); // t=300: тик ещё выполняется — повторно не запускается
    expect(order).toEqual([1]);

    await vi.advanceTimersByTimeAsync(150); // t=450: следующий запуск после завершения тика
    expect(order).toEqual([1, 1]);
  });

  it('errorHandler: ошибка тика маршрутизируется, следующий тик продолжается', async () => {
    vi.useFakeTimers();
    const s = new PollScheduler(makeEnv());
    const errors: string[] = [];
    let n = 0;
    s.add(
      100,
      () => {
        n++;
        if (n === 1) throw new Error('boom');
      },
      { immediate: false, errorHandler: (e) => errors.push(e.message) },
    );
    await vi.advanceTimersByTimeAsync(100);
    expect(errors).toEqual(['boom']);
    await vi.advanceTimersByTimeAsync(100);
    expect(n).toBe(2);
  });

  it('некорректный cron → CronParseError', () => {
    const s = new PollScheduler(makeEnv());
    expect(() => s.add('7 7 7 7 7', vi.fn())).toThrow(/6 полей/);
  });
});