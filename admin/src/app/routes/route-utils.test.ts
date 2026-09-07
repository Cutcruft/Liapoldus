import { describe, expect, it } from 'vitest';
import { isAnchoredRegex, isValidRegex, normalizeAction, overlapWarnings, validateRouteFields, REDIRECT_STATUSES } from './route-utils';
import type { Route } from '../../runtime';

const route = (id: string, matcher: string): Route => ({
  id,
  siteId: 's1',
  matcher,
  priority: 1,
  action: { type: 'renderPage', pageId: 'p1' },
});

describe('isValidRegex', () => {
  it('принимает валидный regex', () => {
    expect(isValidRegex('/about')).toBe(true);
    expect(isValidRegex('/about/:id?')).toBe(true);
    expect(isValidRegex('^/(foo|bar)$')).toBe(true);
  });

  it('отклоняет пустую строку и невалидный regex', () => {
    expect(isValidRegex('')).toBe(false);
    expect(isValidRegex('   ')).toBe(false);
    expect(isValidRegex('(')).toBe(false);
    expect(isValidRegex('*')).toBe(false);
  });
});

describe('isAnchoredRegex', () => {
  it('признаёт якорные ^…$', () => {
    expect(isAnchoredRegex('^/about$')).toBe(true);
    expect(isAnchoredRegex('^/(foo|bar)$')).toBe(true);
    expect(isAnchoredRegex('  ^/about$  ')).toBe(true);
  });

  it('отклоняет неякорные', () => {
    expect(isAnchoredRegex('/about')).toBe(false);
    expect(isAnchoredRegex('^/about')).toBe(false);
    expect(isAnchoredRegex('/about$')).toBe(false);
  });
});

describe('validateRouteFields', () => {
  const base = { matcher: '^/about$', target: '/posts', actionType: 'renderPage' as const, status: '' };

  it('важно без ошибок', () => {
    expect(validateRouteFields(base)).toEqual({ ok: true });
  });

  it('пустой matcher → matcherRequired', () => {
    const res = validateRouteFields({ ...base, matcher: '' });
    expect(res.ok).toBe(false);
    expect(res.matcher).toBe('route.errors.matcherRequired');
  });

  it('невалидный regex matcher → matcherRegex', () => {
    const res = validateRouteFields({ ...base, matcher: '(' });
    expect(res.ok).toBe(false);
    expect(res.matcher).toBe('route.errors.matcherRegex');
  });

  it('неякорный matcher → matcherAnchored', () => {
    const res = validateRouteFields({ ...base, matcher: '/about' });
    expect(res.ok).toBe(false);
    expect(res.matcher).toBe('route.errors.matcherAnchored');
  });

  it('пустой target → targetRequired', () => {
    const res = validateRouteFields({ ...base, target: '' });
    expect(res.ok).toBe(false);
    expect(res.target).toBe('route.errors.targetRequired');
  });

  it('redirect без валидного status → statusRequired', () => {
    const res = validateRouteFields({ ...base, actionType: 'redirect', status: '' });
    expect(res.ok).toBe(false);
    expect(res.status).toBe('route.errors.statusRequired');
  });

  it('redirect с недопустимым статусом (304) → statusRequired', () => {
    const res = validateRouteFields({ ...base, actionType: 'redirect', status: '304' });
    expect(res.ok).toBe(false);
    expect(res.status).toBe('route.errors.statusRequired');
  });

  it('redirect с 302 — валиден', () => {
    const res = validateRouteFields({ ...base, actionType: 'redirect', status: '302' });
    expect(res.ok).toBe(true);
  });
});

describe('REDIRECT_STATUSES', () => {
  it('содержит 301/302/307/308', () => {
    expect(REDIRECT_STATUSES).toEqual([301, 302, 307, 308]);
  });
});

describe('normalizeAction', () => {
  it('redirect: target, status и keepQuery', () => {
    const action = normalizeAction({
      matcher: '/old',
      priority: '1',
      actionType: 'redirect',
      target: ' /new ',
      status: '308',
      keepQuery: true,
    });
    expect(action).toEqual({ type: 'redirect', target: '/new', status: 308, keepQuery: true });
  });

  it('serveAsset: assetId = target', () => {
    expect(
      normalizeAction({ matcher: '/img', priority: '1', actionType: 'serveAsset', target: 'a1', status: '', keepQuery: false }),
    ).toEqual({ type: 'serveAsset', assetId: 'a1' });
  });

  it('renderPage: pageId = target', () => {
    expect(
      normalizeAction({ matcher: '/', priority: '1', actionType: 'renderPage', target: 'p1', status: '', keepQuery: false }),
    ).toEqual({ type: 'renderPage', pageId: 'p1' });
  });
});

describe('overlapWarnings', () => {
  it('точный дубль matcher → duplicate', () => {
    expect(overlapWarnings([route('r1', '/about')], '/about')).toEqual([{ kind: 'duplicate' }]);
  });

  it('префикс: прямой путь пересекает regex, снятый с якорей', () => {
    expect(overlapWarnings([route('r2', '^/about/team$')], '/about')).toEqual([
      { kind: 'prefix', other: '^/about/team$' },
    ]);
  });

  it('префикс: более длинный путь поверх короткого', () => {
    expect(overlapWarnings([route('r1', '/about')], '/about/team/leads')).toEqual([
      { kind: 'prefix', other: '/about' },
    ]);
  });

  it('пропускает исключённый роут (редактирование)', () => {
    expect(overlapWarnings([route('r1', '/about')], '/about', 'r1')).toEqual([]);
  });

  it('без пересечений — пусто', () => {
    expect(overlapWarnings([route('r1', '/pricing')], '/about')).toEqual([]);
  });

  it('пустой matcher — пусто', () => {
    expect(overlapWarnings([route('r1', '/about')], '')).toEqual([]);
  });
});