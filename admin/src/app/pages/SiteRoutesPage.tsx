import { useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { Inline, Stack } from '@liapoldus/ui-kit';
import { runOperation, type Route, type RouteAction, type Translate } from '../../runtime';
import { useAdmin } from '../admin-context';
import { useOperation } from '../use-operation';
import { ConfirmButton } from '../components/ConfirmButton';
import { EntityTable } from '../components/EntityTable';
import { Field } from '../components/Field';
import { isValidRegex, normalizeAction, overlapWarnings, REDIRECT_STATUSES, validateRouteFields } from '../routes/route-utils';

const INPUT_CLASS =
  'rounded border border-neutral-300 px-2 py-1.5 text-sm focus:border-blue-500 focus:outline-none';

function describeAction(t: Translate, action: RouteAction): string {
  switch (action.type) {
    case 'renderPage':
      return `${t('route.action.renderPage')} → ${action.pageId}`;
    case 'serveAsset':
      return `${t('route.action.serveAsset')} → ${action.assetId}`;
    case 'redirect':
      return `${t('route.action.redirect')} → ${action.target}${action.status ? ` ${action.status}` : ''}${
        action.keepQuery ? ' · keepQuery' : ''
      }`;
    default:
      return String(action);
  }
}

export function SiteRoutesPage() {
  const { siteId = '' } = useParams();
  const { api, t } = useAdmin();
  const list = useOperation<Route[]>('listRoutes', { siteId }, (d: unknown) => (Array.isArray(d) ? d : []));

  const [formOpen, setFormOpen] = useState(false);
  const [matcher, setMatcher] = useState('');
  const [priority, setPriority] = useState('1');
  const [actionType, setActionType] = useState<RouteAction['type']>('renderPage');
  const [target, setTarget] = useState('');
  const [status, setStatus] = useState('301');
  const [keepQuery, setKeepQuery] = useState(false);
  const [touched, setTouched] = useState<{ matcher?: boolean; target?: boolean; status?: boolean }>({});
  const [submitting, setSubmitting] = useState(false);
  const [formError, setFormError] = useState('');

  const routes: Route[] = list.state.status === 'success' ? list.state.data : [];

  const check = validateRouteFields({ matcher, target, actionType, status });
  const matcherError = touched.matcher ? check.matcher : undefined;
  const targetError = touched.target ? check.target : undefined;
  const statusError = touched.status ? check.status : undefined;
  const warnings = isValidRegex(matcher) ? overlapWarnings(routes, matcher) : [];

  const markTouched = (key: 'matcher' | 'target' | 'status') =>
    setTouched((prev) => ({ ...prev, [key]: true }));

  const create = async (e: React.FormEvent) => {
    e.preventDefault();
    setFormError('');
    if (!check.ok) {
      setTouched({ matcher: true, target: true, status: true });
      return;
    }
    const action = normalizeAction({ matcher, priority, actionType, target, status, keepQuery });
    setSubmitting(true);
    const res = await runOperation(api, 'createRoute', { siteId, matcher, priority, action }, t);
    setSubmitting(false);
    if (!res.ok) {
      setFormError(res.detail);
      return;
    }
    setMatcher('');
    setTarget('');
    setStatus('301');
    setKeepQuery(false);
    setTouched({});
    setFormOpen(false);
    list.reload();
  };

  const remove = async (route: Route) => {
    const res = await runOperation(api, 'deleteRoute', { siteId, routeId: route.id }, t);
    if (res.ok) list.reload();
  };

  return (
    <Stack pad={8} gap={4}>
      <Inline justify="between" align="center">
        <h1 className="text-xl font-medium">{t('route.title')}</h1>
        <button
          type="button"
          onClick={() => setFormOpen((v) => !v)}
          className="rounded bg-blue-600 px-3 py-1.5 text-sm text-white hover:bg-blue-700"
        >
          {t('route.new')}
        </button>
      </Inline>

      {formOpen && (
        <form onSubmit={create} className="rounded border border-neutral-200 bg-neutral-50 p-4">
          <Stack gap={3}>
            <Inline gap={3} align="end">
              <Field label={t('route.matcher')} required>
                <input
                  className={INPUT_CLASS}
                  value={matcher}
                  onChange={(e) => setMatcher(e.target.value)}
                  onBlur={() => markTouched('matcher')}
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
                  onBlur={() => markTouched('target')}
                />
                {targetError && <p className="text-xs text-red-600">{t(targetError)}</p>}
              </Field>
              {actionType === 'redirect' && (
                <Field label={t('route.redirect.status')} required>
                  <select
                    className={INPUT_CLASS}
                    value={status}
                    onChange={(e) => setStatus(e.target.value)}
                    onBlur={() => markTouched('status')}
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
              <Inline gap={2}>
                <button
                  type="submit"
                  disabled={submitting}
                  className="rounded bg-blue-600 px-3 py-1.5 text-sm text-white hover:bg-blue-700 disabled:opacity-50"
                >
                  {t('common.create')}
                </button>
                <button
                  type="button"
                  onClick={() => setFormOpen(false)}
                  className="rounded border border-neutral-300 px-3 py-1.5 text-sm text-neutral-600"
                >
                  {t('common.cancel')}
                </button>
              </Inline>
            </Inline>
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
            {formError && <p className="text-sm text-red-600">{formError}</p>}
          </Stack>
        </form>
      )}

      {list.state.status === 'loading' && <p className="text-sm text-neutral-400">{t('common.loading')}</p>}

      {list.state.status === 'error' && (
        <Stack gap={2}>
          <p className="text-sm text-red-600">
            {t('common.error')}: {list.state.detail}
          </p>
          <button type="button" onClick={list.reload} className="w-fit rounded border border-neutral-300 px-3 py-1 text-sm">
            {t('common.reload')}
          </button>
        </Stack>
      )}

      {list.state.status === 'success' &&
        (routes.length === 0 ? (
          <p className="text-sm text-neutral-400">{t('route.none')}</p>
        ) : (
          <EntityTable<Route>
            gridClass="grid-cols-[minmax(0,3fr)_minmax(0,5rem)_minmax(0,4fr)_minmax(0,10rem)]"
            getKey={(r: Route) => r.id}
            columns={[
              {
                key: 'matcher',
                label: t('route.matcher'),
                render: (r: Route) => <code className="text-xs">{r.matcher}</code>,
              },
              { key: 'priority', label: t('route.priority'), render: (r: Route) => String(r.priority) },
              { key: 'action', label: t('route.action'), render: (r: Route) => describeAction(t, r.action) },
              {
                key: 'actions',
                label: t('common.actions'),
                className: 'text-right',
                render: (r: Route) => (
                  <Inline gap={3} className="justify-end">
                    <Link
                      to={`/sites/${siteId}/routes/${r.id}`}
                      className="text-xs text-blue-600 hover:underline"
                    >
                      {t('route.edit')}
                    </Link>
                    <ConfirmButton label={t('common.delete')} onConfirm={() => remove(r)} />
                  </Inline>
                ),
              },
            ]}
            rows={routes}
          />
        ))}
    </Stack>
  );
}