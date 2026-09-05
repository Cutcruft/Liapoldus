import type { ResolvedRoute, RouteDescriptor } from '../types/descriptor';

export type MatchFunction = (path: string) => ResolvedRoute | null;

/** Разделяет путь на pathname и query-параметры. */
export function splitPath(path: string): { pathname: string; query: Record<string, string> } {
  const qIdx = path.indexOf('?');
  const pathname = qIdx === -1 ? path : path.slice(0, qIdx);
  const query: Record<string, string> = {};
  if (qIdx !== -1) {
    for (const pair of path.slice(qIdx + 1).split('&')) {
      if (!pair) continue;
      const eq = pair.indexOf('=');
      const key = eq === -1 ? pair : pair.slice(0, eq);
      const value = eq === -1 ? '' : pair.slice(eq + 1);
      try {
        query[decodeURIComponent(key)] = decodeURIComponent(value);
      } catch {
        query[key] = value;
      }
    }
  }
  return { pathname, query };
}

/** Подставляет захваченные группы regex в target редиректа (`$1`, `$2`, …). */
export function applyTargetTemplate(target: string, matcherSource: string, pathname: string): string {
  return target.replace(/\$(\d+)/g, (_, idx: string) => {
    const re = new RegExp(matcherSource);
    const m = re.exec(pathname);
    const n = Number(idx);
    return m && m[n] !== undefined ? m[n] : '';
  });
}

/** Дописывает query к target, если тот ещё не содержит знак '?'. */
export function appendQuery(target: string, query: Record<string, string>): string {
  const keys = Object.keys(query);
  if (keys.length === 0) return target;
  const qs = keys.map((k) => `${encodeURIComponent(k)}=${encodeURIComponent(query[k])}`).join('&');
  return target.includes('?') ? `${target}&${qs}` : `${target}?${qs}`;
}

/**
 * Единая функция матчинга для клиентского Router и edge RequestRouter (§12a#7):
 * сортировка по priority (больше = раньше), при равенстве — порядок регистрации.
 */
export function createMatcher(routes: RouteDescriptor[]): MatchFunction {
  const ordered = [...routes].sort((a, b) => b.priority - a.priority);
  return (path: string): ResolvedRoute | null => {
    const { pathname, query } = splitPath(path);
    for (const route of ordered) {
      const re = new RegExp(route.matcher);
      const m = re.exec(pathname);
      if (m === null) continue;
      const params: Record<string, string> = {};
      if (m.groups) {
        for (const [k, v] of Object.entries(m.groups)) {
          if (v !== undefined) params[k] = v;
        }
      }
      return { route, params, query };
    }
    return null;
  };
}