import { CronParseError } from '../errors';

export interface CronExpression {
  /** undefined = "*" (любое значение). Массивы отсортированы по возрастанию. */
  sec?: number[];
  min?: number[];
  hour?: number[];
  dom?: number[];
  mon?: number[];
  dow?: number[];
  raw: string;
}

const MONTH_NAMES: Record<string, number> = {
  JAN: 1, FEB: 2, MAR: 3, APR: 4, MAY: 5, JUN: 6,
  JUL: 7, AUG: 8, SEP: 9, OCT: 10, NOV: 11, DEC: 12,
};

const DOW_NAMES: Record<string, number> = {
  SUN: 0, MON: 1, TUE: 2, WED: 3, THU: 4, FRI: 5, SAT: 6,
};

function normalizeNames(value: string, names: Record<string, number>): string {
  return value
    .split(',')
    .map((part) => {
      const m = /^([A-Za-z]{3})/.exec(part);
      if (m && names[m[0].toUpperCase()] !== undefined) {
        const num = names[m[0].toUpperCase()];
        return part.replace(m[0], String(num));
      }
      return part;
    })
    .join(',');
}

function parseField(spec: string, min: number, max: number, names?: Record<string, number>): number[] | undefined {
  let field = spec.trim();
  if (field === '*') return undefined;
  if (names) field = normalizeNames(field, names);

  const out = new Set<number>();
  for (const part of field.split(',')) {
    let m = /^(\*|\d+)(?:-(\d+|\*))?(?:\/(\d+))?$/.exec(part);
    let step = 1;
    if (!m) {
      const slash = /^(\d+)-(\d+)\/(\d+)$/.exec(part);
      if (slash) {
        m = /^(\d+)-(\d+)(?:\/(\d+))?$/.exec(`${slash[1]}-${slash[2]}`);
        m = /^(\d+)-(\d+)(?:\/(\d+))?$/.exec(`${slash[1]}-${slash[2]}/${slash[3]}`);
        step = Number(slash[3]);
      }
    }
    if (!m) throw new CronParseError(`Некорректное поле cron '${spec}'`);

    let start: number;
    let end: number;
    if (m[1] === '*') {
      start = min;
      end = m[2] && m[2] !== '*' ? Number(m[2]) : max;
      step = m[3] ? Number(m[3]) : 1;
    } else {
      start = Number(m[1]);
      end = m[2] ? Number(m[2]) : start;
      step = m[3] ? Number(m[3]) : 1;
    }
    if (start < min || end > max || start > end || step < 1) {
      throw new CronParseError(`Значение вне диапазона [${min}..${max}] в '${spec}'`);
    }
    for (let v = start; v <= end; v += step) out.add(v);
  }
  return Array.from(out).sort((a, b) => a - b);
}

/** Парсит 6-полевый cron (sec min hour dom mon dow). Бросает CronParseError. */
export function parseCron(expr: string): CronExpression {
  if (typeof expr !== 'string' || expr.trim().length === 0) {
    throw new CronParseError('Cron-выражение не задано');
  }
  const parts = expr.trim().split(/\s+/);
  if (parts.length !== 6) {
    throw new CronParseError(`Cron должен содержать 6 полей (sec min hour dom mon dow), получено ${parts.length}: '${expr}'`);
  }
  try {
    return {
      sec: parseField(parts[0], 0, 59),
      min: parseField(parts[1], 0, 59),
      hour: parseField(parts[2], 0, 23),
      dom: parseField(parts[3], 1, 31),
      mon: parseField(parts[4], 1, 12, MONTH_NAMES),
      dow: parseField(parts[5], 0, 6, DOW_NAMES),
      raw: expr.trim(),
    };
  } catch (err) {
    if (err instanceof CronParseError) throw err;
    throw new CronParseError(`Не удалось разобрать cron '${expr}': ${err instanceof Error ? err.message : String(err)}`);
  }
}

function matchesMonth(d: Date, mon?: number[]): boolean {
  if (!mon) return true;
  return mon.includes(d.getMonth() + 1);
}

function matchesDowDom(d: Date, dom?: number[], dow?: number[]): boolean {
  if (dom === undefined && dow === undefined) return true;
  if (dom !== undefined && dow !== undefined) {
    return dom.includes(d.getDate()) || dow.includes(d.getDay());
  }
  if (dom !== undefined) return dom.includes(d.getDate());
  return dow!.includes(d.getDay());
}

/** Следующая минута/секунда выполнения после `now` (шаг ≥ 1 сек). */
export function nextTick(now: Date, expr: CronExpression): Date {
  const d = new Date(now.getTime() + 1000);
  d.setMilliseconds(0);
  const guard = 31_536_000; // год в секундах
  for (let i = 0; i < guard; i++) {
    if (!matchesMonth(d, expr.mon)) {
      advanceDay(d);
      continue;
    }
    if (!matchesDowDom(d, expr.dom, expr.dow)) {
      advanceDay(d);
      continue;
    }
    if (expr.hour && !expr.hour.includes(d.getHours())) {
      advanceHour(d);
      continue;
    }
    if (expr.min && !expr.min.includes(d.getMinutes())) {
      advanceMinute(d);
      continue;
    }
    if (expr.sec && !expr.sec.includes(d.getSeconds())) {
      d.setSeconds(d.getSeconds() + 1);
      continue;
    }
    return d;
  }
  throw new CronParseError('Не удалось найти следующую точку выполнения cron');
}

function advanceDay(d: Date): void {
  d.setDate(d.getDate() + 1);
  d.setHours(0, 0, 0, 0);
}

function advanceHour(d: Date): void {
  d.setHours(d.getHours() + 1, 0, 0, 0);
}

function advanceMinute(d: Date): void {
  d.setMinutes(d.getMinutes() + 1, 0, 0);
}