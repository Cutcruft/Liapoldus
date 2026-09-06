import type { Route, RouteAction } from '../../runtime';

export const REDIRECT_STATUSES = [301, 302, 307, 308] as const;

export function isValidRegex(value: string): boolean {
  if (!value.trim()) return false;
  try {
    new RegExp(value);
    return true;
  } catch {
    return false;
  }
}

export type RouteFields = {
  matcher: string;
  priority: string;
  actionType: RouteAction['type'];
  target: string;
  status: string;
  keepQuery: boolean;
};

export type RouteValidationResult = {
  ok: boolean;
  matcher?: string;
  target?: string;
  status?: string;
};

export function validateRouteFields(
  f: Pick<RouteFields, 'matcher' | 'target' | 'actionType' | 'status'>,
): RouteValidationResult {
  const out: RouteValidationResult = { ok: true };
  if (!f.matcher.trim()) out.matcher = tKey('matcherRequired');
  else if (!isValidRegex(f.matcher)) out.matcher = tKey('matcherRegex');
  if (!f.target.trim()) out.target = tKey('targetRequired');
  const statusNum = Number(f.status);
  if (f.actionType === 'redirect' && !(REDIRECT_STATUSES as readonly number[]).includes(statusNum)) {
    out.status = tKey('statusRequired');
  }
  out.ok = !out.matcher && !out.target && !out.status;
  return out;
}

function tKey(k: string): string {
  return `route.errors.${k}`;
}

export function normalizeAction(f: RouteFields): RouteAction {
  switch (f.actionType) {
    case 'redirect':
      return { type: 'redirect', target: f.target.trim(), status: Number(f.status), keepQuery: f.keepQuery };
    case 'serveAsset':
      return { type: 'serveAsset', assetId: f.target.trim() };
    default:
      return { type: 'renderPage', pageId: f.target.trim() };
  }
}

export type OverlapWarning =
  | { kind: 'duplicate' }
  | { kind: 'prefix'; other: string };

function anchorFree(value: string): string {
  return value.trim().replace(/^\^/, '').replace(/\$$/, '');
}

export function overlapWarnings(routes: Route[], matcher: string, excludeId?: string): OverlapWarning[] {
  const m = anchorFree(matcher);
  if (!m) return [];
  const warnings: OverlapWarning[] = [];
  for (const r of routes) {
    if (excludeId && r.id === excludeId) continue;
    const other = anchorFree(r.matcher);
    if (!other) continue;
    if (other === m) {
      warnings.push({ kind: 'duplicate' });
    } else if (other.length >= 3 && m.startsWith(other)) {
      warnings.push({ kind: 'prefix', other: r.matcher });
    } else if (m.length >= 3 && other.startsWith(m)) {
      warnings.push({ kind: 'prefix', other: r.matcher });
    }
  }
  return warnings;
}