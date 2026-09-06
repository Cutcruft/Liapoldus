import { useEffect, useState } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { Inline, Stack } from '@liapoldus/ui-kit';
import { runOperation, type Route, type RouteAction } from '../../runtime';
import { useAdmin } from '../admin-context';
import { useOperation } from '../use-operation';
import { Field } from '../components/Field';
import { ConfirmButton } from '../components/ConfirmButton';
import {
  isValidRegex,
  normalizeAction,
  overlapWarnings,
  REDIRECT_STATUSES,
  validateRouteFields,
} from '../routes/route-utils';

const INPUT_CLASS =
  'rounded border border-neutral-300 px-2 py-1.5 text-sm focus:border-blue-500 focus:outline-none';

export function RouteEditorPage() {
  const { siteId = '', routeId = '' } = useParams();
  const navigate = useNavigate();
  const { api, t } = useAdmin();

  const item = useOperation<Route | null>(
    'getRoute',
    { siteId, routeId },
    (d) => (typeof d === 'object' && d !== null && 'matcher' in d ? (d as Route) : null),
  );
  const all = useOperation<Route[]>('listRoutes', { siteId }, (d: unknown) => (Array.isArray(d) ? d : []));

  const [matcher, setMatcher] = useState('');
  const [priority, setPriority] = useState('1');
  const [actionType, setActionType] = useState<RouteAction['type']>('renderPage');
  const [target, setTarget] = useState('');
  const [status, setStatus] = useState('301');
  const [keepQuery, setKeepQuery] = useState(false);
  const [touched, setTouched] = useState<{ matcher?: boolean; target?: boolean; status?: boolean }>({});
  const [saved, setSaved] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (item.state.status !== 'success' || !item.state.data) return;
    const r = item.state.data;
    setMatcher(r.matcher);
    setPriority(String(r.priority));
    setActionType(r.action.type);
    setTarget(r.action.type === 'redirect' ? r.action.target : r.action.type === 'serveAsset' ? r.action.assetId : r.action.pageId);
    setStatus(r.action.type === 'redirect' ? String(r.action.status ?? 301) : '301');
    setKeepQuery(r.action.type === 'redirect' ? Boolean(r.action.keepQuery) : false);
    setSaved('');
  }, [item.state]);

  const routes: Route[] = all.state.status === 'success' ? all.state.data : [];
  const check = validateRouteFields({ matcher, target, actionType, status });
  const matcherError = touched.matcher ? check.matcher : undefined;
  const targetError = touched.target ? check.target : undefined;
  const statusError = touched.status ? check.status : undefined;
  const warnings = isValidRegex(matcher) ? overlapWarnings(routes, matcher, routeId) : [];

  const save = async () => {
    setError('');
    if (!check.ok) {
      setTouched({ matcher: true, target: true, status: true });
      return;
    }
    const action = normalizeAction({ matcher, priority, actionType, target, status, keepQuery });
    setBusy(true);
    const res = await runOperation(
      api,
      'updateRoute',
      { siteId, routeId, patch: { matcher, priority: Number(priority), action } },
      t,
    );
    setBusy(false);
    if (res.ok) setSaved(t('route.saved'));
    else setError(res.detail);
  };

  const remove = async () => {
    const res = await runOperation(api, 'deleteRoute', { siteId, routeId }, t);
    if (res.ok) navigate(`/sites/${siteId}/routes`);
  };

  if (item.state.status === 'loading') {
    return <p className="p-8 text-sm text-neutral-400">{t('common.loading')}</p>;
  }

  if (item.state.status === 'error') {
    return (
      <Stack pad={8} gap={2}>
        <p className="text-sm text-red-600">
          {t('common.error')}: {item.state.detail}
        </p>
        <button type="button" onClick={item.reload} className="w-fit rounded border border-neutral-300 px-3 py-1 text-sm">
          {t('common.reload')}
        </button>
      </Stack>
    );
  }

  return (
    <Stack pad={8} gap={4}>
      <Inline justify="between" align="center">
        <div>
          <h1 className="text-xl font-medium">{t('route.title')}</h1>
          <p className="text-sm text-neutral-500">
            <Link to={`/sites/${siteId}/routes`} className="text-blue-600 hover:underline">
              {t('route.back')}
            </Link>
            <span> · </span>
            <code className="font-mono text-xs">{item.state.data!.id}</code>
          </p>
        </div>
        <div className="flex items-center gap-3 text-sm">
          {saved && <span className="text-emerald-600">{saved}</span>}
          {error && <span className="text-red-600">{error}</span>}
        </div>
      </Inline>

      <Stack gap={3} className="max-w-xl">
        <Field label={t('route.matcher')} required>
          <input
            className={INPUT_CLASS}
            value={matcher}
            onChange={(e) => setMatcher(e.target.value)}
            onBlur={() => setTouched((p) => ({ ...p, matcher: true }))}
          />
          {matcherError && <p className="text-xs text-red-600">{t(matcherError)}</p>}
        </Field>
        <Field label={t('route.priority')}>
          <input
            className={INPUT_CLASS}
            type="number"
            min={0}
            value={priority}
            onChange={(e) => setPriority(e.target.value)}
          />
        </Field>
        <Field label={t('route.action')}>
          <select
            className={INPUT_CLASS}
            value={actionType}
            onChange={(e) => setActionType(e.target.value as RouteAction['type'])}
          >
            <option value="renderPage">{t('route.action.renderPage')}</option>
            <option value="serveAsset">{t('route.action.serveAsset')}</option>
            <option value="redirect">{t('route.action.redirect')}</option>
          </select>
        </Field>
        <Field label={t('route.target')} required>
          <input
            className={INPUT_CLASS}
            value={target}
            onChange={(e) => setTarget(e.target.value)}
            onBlur={() => setTouched((p) => ({ ...p, target: true }))}
          />
          {targetError && <p className="text-xs text-red-600">{t(targetError)}</p>}
        </Field>
        {actionType === 'redirect' && (
          <Field label={t('route.redirect.status')} required>
            <select
              className={INPUT_CLASS}
              value={status}
              onChange={(e) => setStatus(e.target.value)}
              onBlur={() => setTouched((p) => ({ ...p, status: true }))}
            >
              {REDIRECT_STATUSES.map((s) => (
                <option key={s} value={s}>
                  {s}
                </option>
              ))}
            </select>
            {statusError && <p className="text-xs text-red-600">{t(statusError)}</p>}
          </Field>
        )}
        {actionType === 'redirect' && (
          <Field label={t('route.redirect.keepQuery')}>
            <label className="flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                checked={keepQuery}
                onChange={(e) => setKeepQuery(e.target.checked)}
                className="size-4 accent-blue-600"
              />
              {t('route.redirect.keepQuery')}
            </label>
          </Field>
        )}
        {warnings.length > 0 && (
          <Stack gap={1} className="rounded border border-amber-300 bg-amber-50 p-2 text-xs text-amber-800">
            {warnings.map((w, i) => (
              <p key={i}>
                {w.kind === 'duplicate'
                  ? t('route.warnings.duplicate')
                  : t('route.warnings.prefix', { matcher: w.other })}
              </p>
            ))}
          </Stack>
        )}
        <Inline gap={2} align="center">
          <button
            type="button"
            onClick={save}
            disabled={busy}
            className="rounded bg-blue-600 px-3 py-1.5 text-sm text-white hover:bg-blue-700 disabled:opacity-50"
          >
            {t('route.save')}
          </button>
          <ConfirmButton label={t('route.delete.confirm')} onConfirm={remove} />
        </Inline>
      </Stack>
    </Stack>
  );
}