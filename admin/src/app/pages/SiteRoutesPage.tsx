import { useState } from 'react';
import { useParams } from 'react-router-dom';
import { Inline, Stack } from '@liapoldus/ui-kit';
import { runOperation, type Route, type RouteAction, type Translate } from '../../runtime';
import { useAdmin } from '../admin-context';
import { useOperation } from '../use-operation';
import { ConfirmButton } from '../components/ConfirmButton';
import { EntityTable } from '../components/EntityTable';
import { Field } from '../components/Field';

const INPUT_CLASS =
  'rounded border border-neutral-300 px-2 py-1.5 text-sm focus:border-blue-500 focus:outline-none';

function describeAction(t: Translate, action: RouteAction): string {
  switch (action.type) {
    case 'renderPage':
      return `${t('route.action.renderPage')} → ${action.pageId}`;
    case 'serveAsset':
      return `${t('route.action.serveAsset')} → ${action.assetId}`;
    case 'redirect':
      return `${t('route.action.redirect')} → ${action.target}`;
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
  const [submitting, setSubmitting] = useState(false);
  const [formError, setFormError] = useState('');

  const create = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!matcher.trim() || !target.trim()) {
      setFormError(t('common.required'));
      return;
    }
    const action: RouteAction =
      actionType === 'redirect'
        ? { type: 'redirect', target }
        : actionType === 'serveAsset'
          ? { type: 'serveAsset', assetId: target }
          : { type: 'renderPage', pageId: target };
    setSubmitting(true);
    setFormError('');
    const res = await runOperation(api, 'createRoute', { siteId, matcher, priority, action }, t);
    setSubmitting(false);
    if (!res.ok) {
      setFormError(res.detail);
      return;
    }
    setMatcher('');
    setTarget('');
    setFormOpen(false);
    list.reload();
  };

  const remove = async (route: Route) => {
    const res = await runOperation(api, 'deleteRoute', { siteId, routeId: route.id }, t);
    if (res.ok) list.reload();
  };

  const routes: Route[] = list.state.status === 'success' ? list.state.data : [];

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
                <input className={INPUT_CLASS} value={matcher} onChange={(e) => setMatcher(e.target.value)} />
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
                <input className={INPUT_CLASS} value={target} onChange={(e) => setTarget(e.target.value)} />
              </Field>
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
            gridClass="grid-cols-[minmax(0,3fr)_minmax(0,5rem)_minmax(0,3fr)_minmax(0,8rem)]"
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
                  <ConfirmButton label={t('common.delete')} onConfirm={() => remove(r)} />
                ),
              },
            ]}
            rows={routes}
          />
        ))}
    </Stack>
  );
}