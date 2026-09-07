import { useMemo, useState } from 'react';
import { useParams, useSearchParams } from 'react-router-dom';
import { Inline, Stack } from '@liapoldus/ui-kit';
import {
  runOperation,
  type AdminEndpoint,
  type AdminForm,
  type AdminOperation,
  type OperationParamSpec,
} from '../../runtime';
import { useAdmin } from '../admin-context';
import { useOperation } from '../use-operation';
import { ConfirmButton } from '../components/ConfirmButton';
import { EntityTable, type Column } from '../components/EntityTable';
import { Field } from '../components/Field';

const INFRA_TABS = ['operations', 'endpoints', 'forms'] as const;
type InfraTab = (typeof INFRA_TABS)[number];

const METHOD_SELECT = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE'] as const;
const CACHE_SELECT = ['immutable', 'disabled', 'ttl'] as const;

function isInfraTab(v: string | null): v is InfraTab {
  return (INFRA_TABS as readonly string[]).includes(v ?? '');
}

/** Относительный путь runtime (`/api/...`). Абсолютные URL не поддерживаются. */
function isRelativePath(path: string): boolean {
  return path.startsWith('/') && !path.startsWith('//') && !/^https?:\/\//i.test(path);
}

function SystemBadge({ system }: { system: boolean }) {
  const { t } = useAdmin();
  return (
    <span
      className={
        system
          ? 'rounded border border-neutral-200 bg-neutral-100 px-1.5 py-0.5 text-[11px] text-neutral-500'
          : 'rounded border border-blue-200 bg-blue-50 px-1.5 py-0.5 text-[11px] text-blue-700'
      }
    >
      {system ? t('infra.systemBadge') : t('infra.editableBadge')}
    </span>
  );
}

const inputClass =
  'w-full rounded border border-neutral-300 bg-white px-2 py-1.5 text-sm text-neutral-900 focus:border-blue-500 focus:outline-none';

/**
 * Раздел «Инфраструктура» (R6): операции и эндпоинты ui-runtime из БД
 * (админ-CRUD; системные дескрипторы — read-only) + редактор определений
 * форм (fields + submit.target из операций/эндпоинтов). Подтабы в URL
 * `?section=infra&infraTab=...`.
 */
export function InfraSection() {
  const { t } = useAdmin();
  const [searchParams, setSearchParams] = useSearchParams();
  const tab: InfraTab = isInfraTab(searchParams.get('infraTab')) ? (searchParams.get('infraTab') as InfraTab) : 'operations';

  const pick = (next: InfraTab) => {
    const p = new URLSearchParams(searchParams);
    p.set('infraTab', next);
    setSearchParams(p);
  };

  return (
    <Stack gap={4} className="h-full min-h-0">
      <div className="flex items-center gap-2" role="tablist" aria-label="Инфраструктура">
        {INFRA_TABS.map((key) => (
          <button
            key={key}
            type="button"
            role="tab"
            aria-selected={tab === key}
            onClick={() => pick(key)}
            className={`rounded-md px-3 py-1.5 text-sm ${
              tab === key ? 'bg-blue-50 font-medium text-blue-800' : 'text-neutral-600 hover:bg-neutral-100'
            }`}
          >
            {t(`infra.${key}`)}
          </button>
        ))}
      </div>
      {tab === 'operations' ? <OperationsTab /> : tab === 'endpoints' ? <EndpointsTab /> : <FormsTab />}
    </Stack>
  );
}

// --- Операции --------------------------------------------------------------

type OpDraft = {
  id: string;
  provider: string;
  typeOp: 'query' | 'mutation';
  method: string;
  path: string;
  cache: string;
  ttl: string;
  scope: string;
  resultType: string;
  paramsText: string;
};

function opToDraft(op: AdminOperation): OpDraft {
  return {
    id: op.id,
    provider: op.provider ?? '',
    typeOp: op.typeOp === 'mutation' ? 'mutation' : 'query',
    method: op.method,
    path: op.path ?? '',
    cache: op.cache ?? 'disabled',
    ttl: op.ttl != null ? String(op.ttl) : '',
    scope: op.scope ?? 'public',
    resultType: op.resultType ?? '',
    paramsText: op.params ? JSON.stringify(op.params, null, 2) : '',
  };
}

function OperationEditor({
  siteId,
  initial,
  onClose,
  onSaved,
}: {
  siteId: string;
  initial?: AdminOperation | null;
  onClose: () => void;
  onSaved: () => void;
}) {
  const { api, t } = useAdmin();
  const editing = initial != null;
  const [draft, setDraft] = useState<OpDraft>(opToDraft(initial ?? ({} as AdminOperation)));
  const [error, setError] = useState('');
  const [saving, setSaving] = useState(false);

  const set = <K extends keyof OpDraft>(key: K, value: OpDraft[K]) => setDraft((d) => ({ ...d, [key]: value }));

  const save = async () => {
    setError('');
    if (!isRelativePath(draft.path)) {
      setError(t('infra.path.invalid'));
      return;
    }
    let params: Record<string, unknown> | undefined;
    if (draft.paramsText.trim()) {
      try {
        const parsed = JSON.parse(draft.paramsText);
        if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) {
          setError('params — объект JSON');
          return;
        }
        params = parsed as Record<string, unknown>;
      } catch (e) {
        setError(e instanceof Error ? e.message : String(e));
        return;
      }
    }
    const ttl = draft.cache === 'ttl' ? Number(draft.ttl) || undefined : undefined;
    const base = {
      provider: draft.provider,
      typeOp: draft.typeOp,
      method: draft.method,
      path: draft.path,
      cache: draft.cache,
      ttl,
      scope: draft.scope,
      resultType: draft.resultType,
      params: params ?? {},
      poll: {},
      subscribe: {},
    };
    setSaving(true);
    const res = editing
      ? await runOperation(api, 'updateOperation', { siteId, operationId: draft.id, ...base }, t)
      : await runOperation(api, 'createOperation', { siteId, ...base, id: draft.id }, t);
    setSaving(false);
    if (!res.ok) {
      setError(res.detail);
      return;
    }
    onSaved();
  };

  return (
    <div className="rounded border border-neutral-200 bg-white p-4">
      <h3 className="mb-3 text-sm font-medium text-neutral-900">
        {editing ? t('infra.editOperation') : t('infra.newOperation')}
      </h3>
      <Inline gap={3} wrap>
        {!editing && (
          <Field label={t('infra.operation.id')} required>
            <input className={inputClass} value={draft.id} onChange={(e) => set('id', e.target.value)} />
          </Field>
        )}
        <Field label={t('infra.operation.provider')}>
          <input
            className={inputClass}
            value={draft.provider}
            placeholder="content.api"
            onChange={(e) => set('provider', e.target.value)}
          />
        </Field>
        <Field label={t('infra.operation.typeOp')}>
          <select
            className={inputClass}
            value={draft.typeOp}
            onChange={(e) => set('typeOp', e.target.value as OpDraft['typeOp'])}
          >
            <option value="query">query</option>
            <option value="mutation">mutation</option>
          </select>
        </Field>
        <Field label={t('infra.operation.method')}>
          <select className={inputClass} value={draft.method} onChange={(e) => set('method', e.target.value)}>
            {METHOD_SELECT.map((m) => (
              <option key={m} value={m}>
                {m}
              </option>
            ))}
          </select>
        </Field>
        <Field label={t('infra.operation.path')} required>
          <input
            className={inputClass}
            value={draft.path}
            placeholder="/api/items/{id}"
            onChange={(e) => set('path', e.target.value)}
          />
        </Field>
        <Field label={t('infra.operation.cache')}>
          <select className={inputClass} value={draft.cache} onChange={(e) => set('cache', e.target.value)}>
            {CACHE_SELECT.map((c) => (
              <option key={c} value={c}>
                {c}
              </option>
            ))}
          </select>
        </Field>
        {draft.cache === 'ttl' && (
          <Field label="TTL">
            <input className={inputClass} value={draft.ttl} onChange={(e) => set('ttl', e.target.value)} />
          </Field>
        )}
        <Field label={t('infra.operation.scope')}>
          <select className={inputClass} value={draft.scope} onChange={(e) => set('scope', e.target.value)}>
            <option value="public">public</option>
            <option value="server">server</option>
          </select>
        </Field>
        <Field label={t('infra.operation.resultType')}>
          <input
            className={inputClass}
            value={draft.resultType}
            placeholder="content[]"
            onChange={(e) => set('resultType', e.target.value)}
          />
        </Field>
      </Inline>
      <Field label="params (JSON)">
        <textarea
          className={`${inputClass} font-mono text-xs`}
          rows={4}
          value={draft.paramsText}
          placeholder={'{"in":"query","fields":{"cursor":{"required":false}}}'}
          onChange={(e) => set('paramsText', e.target.value)}
        />
      </Field>
      {error && <p className="mt-2 text-sm text-red-600">{error}</p>}
      <Inline gap={2} className="mt-3">
        <button
          type="button"
          onClick={save}
          disabled={saving}
          className="rounded bg-blue-600 px-3 py-1.5 text-sm text-white hover:bg-blue-700 disabled:opacity-60"
        >
          {saving ? t('infra.save') + '…' : editing ? t('infra.save') : t('infra.create')}
        </button>
        <button type="button" onClick={onClose} className="rounded border border-neutral-300 px-3 py-1.5 text-sm">
          Отмена
        </button>
      </Inline>
    </div>
  );
}

function OperationPreview({ siteId, op }: { siteId: string; op: AdminOperation }) {
  const { api, t } = useAdmin();
  const [values, setValues] = useState<Record<string, string>>({});
  const [bodyText, setBodyText] = useState('');
  const [response, setResponse] = useState<{ ok: boolean; body: unknown } | null>(null);
  const [pending, setPending] = useState(false);
  const [invalid, setInvalid] = useState(false);

  const spec: OperationParamSpec | undefined = op.params as OperationParamSpec | undefined;
  const fields = spec?.fields ?? {};
  const fieldNames = Object.keys(fields);
  const usesBody = spec?.in === 'body';

  const setValue = (name: string, value: string) => setValues((v) => ({ ...v, [name]: value }));

  const request = async () => {
    setInvalid(false);
    if (!isRelativePath(op.path ?? '')) {
      setInvalid(true);
      return;
    }
    setPending(true);
    let body: unknown = undefined;
    if (usesBody) {
      try {
        body = bodyText.trim() ? JSON.parse(bodyText) : undefined;
      } catch (e) {
        setResponse({ ok: false, body: { error: e instanceof Error ? e.message : String(e) } });
        setPending(false);
        return;
      }
    }
    const pathValues: Record<string, unknown> = {};
    const query: Record<string, unknown> = {};
    for (const name of fieldNames) {
      const value = values[name] ?? '';
      if (spec?.in === 'query') query[name] = value;
      else pathValues[name] = value;
    }
    const url = api.pathWithQuery(op.path ?? '/', { ...pathValues, ...query }, query);
    const res = await api.request(op.method as 'GET' | 'POST' | 'PUT' | 'DELETE', url, body);
    setResponse({ ok: res.ok, body: res.ok ? res.body : res.body ?? res.error?.message });
    setPending(false);
  };

  const run = () => {
    if (op.typeOp === 'mutation') {
      if (!window.confirm(t('infra.preview.mutation.confirm', { op: op.id }))) return;
    }
    void request();
  };

  return (
    <div className="rounded border border-neutral-200 bg-neutral-50 p-4">
      <div className="flex items-center justify-between gap-3">
        <h3 className="text-sm font-medium text-neutral-900">
          {t('infra.preview')}: {op.id}
        </h3>
        <code className="rounded bg-neutral-100 px-2 py-0.5 text-xs text-neutral-600">
          {op.method} {op.path}
        </code>
      </div>

      {fieldNames.length > 0 && !usesBody ? (
        <Inline gap={2} wrap className="mt-3">
          {fieldNames.map((name) => (
            <Field key={name} label={name} required={fields[name]?.required}>
              <input className={inputClass} value={values[name] ?? ''} onChange={(e) => setValue(name, e.target.value)} />
            </Field>
          ))}
        </Inline>
      ) : usesBody ? (
        <Field label={t('infra.preview.params.title')} className="mt-3">
          <textarea
            className={`${inputClass} font-mono text-xs`}
            rows={3}
            placeholder="JSON body"
            value={bodyText}
            onChange={(e) => setBodyText(e.target.value)}
          />
        </Field>
      ) : (
        <p className="mt-3 text-sm text-neutral-500">{t('infra.preview.params.none')}</p>
      )}

      {invalid && <p className="mt-2 text-sm text-red-600">{t('infra.path.invalid')}</p>}

      <Inline gap={2} className="mt-3">
        <button
          type="button"
          onClick={run}
          disabled={pending}
          className="rounded bg-neutral-900 px-3 py-1.5 text-sm text-white hover:bg-neutral-700 disabled:opacity-60"
        >
          {pending ? t('infra.preview.run.pending') : t('infra.preview.run')}
        </button>
      </Inline>

      <div className="mt-3">
        <p className="mb-1 text-xs font-medium text-neutral-500">{t('infra.preview.response.title')}</p>
        {response ? (
          <pre className="max-h-64 overflow-auto rounded border border-neutral-200 bg-white p-3 text-xs text-neutral-800">
            {JSON.stringify(response.body, null, 2)}
          </pre>
        ) : (
          <p className="text-xs text-neutral-400">{t('infra.preview.response.none')}</p>
        )}
      </div>
    </div>
  );
}

function OperationsTab() {
  const { siteId = '' } = useParams();
  const { api, t } = useAdmin();
  const list = useOperation<AdminOperation[]>(
    'listOperations',
    { siteId },
    (d) => (Array.isArray(d) ? (d as AdminOperation[]) : []),
  );
  const [creating, setCreating] = useState(false);
  const [editingOp, setEditingOp] = useState<AdminOperation | null>(null);
  const [previewId, setPreviewId] = useState<string | null>(null);

  const ops = list.state.status === 'success' ? list.state.data : [];
  const previewOp = ops.find((o) => o.id === previewId) ?? null;

  const columns: Column<AdminOperation>[] = [
    {
      key: 'id',
      label: t('infra.operation.id'),
      className: 'font-medium text-neutral-900',
      render: (o) => (
        <Inline gap={2}>
          <span>{o.id}</span>
          <SystemBadge system={o.system} />
        </Inline>
      ),
    },
    { key: 'typeOp', label: t('infra.operation.typeOp'), render: (o) => o.typeOp },
    { key: 'method', label: t('infra.operation.method'), render: (o) => o.method },
    { key: 'path', label: t('infra.operation.path'), className: 'font-mono text-xs', render: (o) => o.path },
    { key: 'cache', label: t('infra.operation.cache'), render: (o) => (o.cache === 'ttl' && o.ttl ? `${o.cache} ${o.ttl}s` : o.cache) },
    { key: 'actions', label: '', render: (o) => (
        <Inline gap={2}>
          <button type="button" onClick={() => setPreviewId(o.id)} className="text-sm text-blue-700 hover:underline">
            {t('infra.preview')}
          </button>
          <button
            type="button"
            disabled={o.system}
            onClick={() => { setEditingOp(o); setCreating(false); }}
            className="text-sm text-neutral-600 hover:underline disabled:cursor-not-allowed disabled:text-neutral-300"
          >
            {t('infra.editOperation')}
          </button>
          {!o.system && (
            <ConfirmButton
              onConfirm={async () => {
                const res = await runOperation(api, 'deleteOperation', { siteId, operationId: o.id }, t);
                if (res.ok) list.reload();
              }}
              label={t('infra.delete')}
            />
          )}
        </Inline>
      ),
    },
  ];

  return (
    <Stack gap={3}>
      <Inline gap={2}>
        <button
          type="button"
          onClick={() => { setCreating(true); setEditingOp(null); }}
          className="w-fit rounded bg-blue-600 px-3 py-1.5 text-sm text-white hover:bg-blue-700"
        >
          {t('infra.newOperation')}
        </button>
        <button type="button" onClick={list.reload} className="w-fit rounded border border-neutral-300 px-3 py-1.5 text-sm">
          {t('common.reload')}
        </button>
      </Inline>

      {(creating || editingOp) && (
        <OperationEditor
          siteId={siteId}
          initial={editingOp}
          onClose={() => { setCreating(false); setEditingOp(null); }}
          onSaved={() => { setCreating(false); setEditingOp(null); list.reload(); }}
        />
      )}

      {list.state.status === 'loading' ? (
        <p className="text-sm text-neutral-400">{t('common.loading')}</p>
      ) : list.state.status === 'error' ? (
        <p className="text-sm text-red-600">
          {t('common.error')}: {list.state.detail}
        </p>
      ) : ops.length === 0 ? (
        <p className="text-sm text-neutral-500">{t('infra.none.operations')}</p>
      ) : (
        <EntityTable columns={columns} rows={ops} getKey={(o) => o.provider + ':' + o.id} gridClass="grid-cols-[minmax(0,1fr)_minmax(0,6rem)_minmax(0,5rem)_minmax(0,1fr)_minmax(0,7rem)_minmax(0,14rem)]" />
      )}

      {previewOp && <OperationPreview siteId={siteId} op={previewOp} />}
    </Stack>
  );
}

// --- Эндпоинты --------------------------------------------------------------

type EndpointDraft = { id: string; method: string; path: string; operationId: string };

function EndpointEditor({
  siteId,
  operations,
  initial,
  onClose,
  onSaved,
}: {
  siteId: string;
  operations: AdminOperation[];
  initial?: AdminEndpoint | null;
  onClose: () => void;
  onSaved: () => void;
}) {
  const { api, t } = useAdmin();
  const editing = initial != null;
  const [draft, setDraft] = useState<EndpointDraft>({
    id: initial?.id ?? '',
    method: initial?.method ?? 'POST',
    path: initial?.path ?? '',
    operationId: initial?.operationId ?? '',
  });
  const [error, setError] = useState('');
  const [saving, setSaving] = useState(false);

  const save = async () => {
    setError('');
    if (!isRelativePath(draft.path)) {
      setError(t('infra.path.invalid'));
      return;
    }
    setSaving(true);
    const res = editing
      ? await runOperation(api, 'updateEndpoint', { siteId, endpointId: draft.id, ...draft }, t)
      : await runOperation(api, 'createEndpoint', { siteId, ...draft }, t);
    setSaving(false);
    if (!res.ok) {
      setError(res.detail);
      return;
    }
    onSaved();
  };

  return (
    <div className="rounded border border-neutral-200 bg-white p-4">
      <h3 className="mb-3 text-sm font-medium text-neutral-900">
        {editing ? t('infra.editEndpoint') : t('infra.newEndpoint')}
      </h3>
      <Inline gap={3} wrap>
        {!editing && (
          <Field label={t('infra.endpoint.id')} required>
            <input className={inputClass} value={draft.id} onChange={(e) => setDraft((d) => ({ ...d, id: e.target.value }))} />
          </Field>
        )}
        <Field label={t('infra.endpoint.method')}>
          <select className={inputClass} value={draft.method} onChange={(e) => setDraft((d) => ({ ...d, method: e.target.value }))}>
            {METHOD_SELECT.map((m) => (
              <option key={m} value={m}>
                {m}
              </option>
            ))}
          </select>
        </Field>
        <Field label={t('infra.endpoint.path')} required>
          <input
            className={inputClass}
            value={draft.path}
            placeholder="/booking"
            onChange={(e) => setDraft((d) => ({ ...d, path: e.target.value }))}
          />
        </Field>
        <Field label={t('infra.endpoint.operationId')} required>
          <select className={inputClass} value={draft.operationId} onChange={(e) => setDraft((d) => ({ ...d, operationId: e.target.value }))}>
            <option value="">—</option>
            {operations.map((o) => (
              <option key={o.id} value={o.id}>
                {o.id}
              </option>
            ))}
          </select>
        </Field>
      </Inline>
      {error && <p className="mt-2 text-sm text-red-600">{error}</p>}
      <Inline gap={2} className="mt-3">
        <button
          type="button"
          onClick={save}
          disabled={saving}
          className="rounded bg-blue-600 px-3 py-1.5 text-sm text-white hover:bg-blue-700 disabled:opacity-60"
        >
          {saving ? t('infra.save') + '…' : editing ? t('infra.save') : t('infra.create')}
        </button>
        <button type="button" onClick={onClose} className="rounded border border-neutral-300 px-3 py-1.5 text-sm">
          Отмена
        </button>
      </Inline>
    </div>
  );
}

function EndpointsTab() {
  const { siteId = '' } = useParams();
  const { api, t } = useAdmin();
  const endpoints = useOperation<AdminEndpoint[]>(
    'listEndpoints',
    { siteId },
    (d) => (Array.isArray(d) ? (d as AdminEndpoint[]) : []),
  );
  const ops = useOperation<AdminOperation[]>(
    'listOperations',
    { siteId },
    (d) => (Array.isArray(d) ? (d as AdminOperation[]) : []),
  );
  const [creating, setCreating] = useState(false);
  const [editing, setEditing] = useState<AdminEndpoint | null>(null);

  const rows = endpoints.state.status === 'success' ? endpoints.state.data : [];

  const columns: Column<AdminEndpoint>[] = [
    {
      key: 'id',
      label: t('infra.endpoint.id'),
      className: 'font-medium text-neutral-900',
      render: (e) => (
        <Inline gap={2}>
          <span>{e.id}</span>
          <SystemBadge system={e.system} />
        </Inline>
      ),
    },
    { key: 'method', label: t('infra.endpoint.method'), render: (e) => e.method },
    { key: 'path', label: t('infra.endpoint.path'), className: 'font-mono text-xs', render: (e) => e.path },
    { key: 'operationId', label: t('infra.endpoint.operationId'), render: (e) => e.operationId },
    {
      key: 'actions',
      label: '',
      render: (e) => (
        <Inline gap={2}>
          <button
            type="button"
            disabled={e.system}
            onClick={() => { setEditing(e); setCreating(false); }}
            className="text-sm text-neutral-600 hover:underline disabled:cursor-not-allowed disabled:text-neutral-300"
          >
            {t('infra.editEndpoint')}
          </button>
          {!e.system && (
            <ConfirmButton
              onConfirm={async () => {
                const res = await runOperation(api, 'deleteEndpoint', { siteId, endpointId: e.id }, t);
                if (res.ok) endpoints.reload();
              }}
              label={t('infra.delete')}
            />
          )}
        </Inline>
      ),
    },
  ];

  return (
    <Stack gap={3}>
      <Inline gap={2}>
        <button
          type="button"
          onClick={() => { setCreating(true); setEditing(null); }}
          className="w-fit rounded bg-blue-600 px-3 py-1.5 text-sm text-white hover:bg-blue-700"
        >
          {t('infra.newEndpoint')}
        </button>
        <button type="button" onClick={endpoints.reload} className="w-fit rounded border border-neutral-300 px-3 py-1.5 text-sm">
          {t('common.reload')}
        </button>
      </Inline>

      {(creating || editing) && (
        <EndpointEditor
          siteId={siteId}
          operations={ops.state.status === 'success' ? ops.state.data : []}
          initial={editing}
          onClose={() => { setCreating(false); setEditing(null); }}
          onSaved={() => { setCreating(false); setEditing(null); endpoints.reload(); }}
        />
      )}

      {endpoints.state.status === 'loading' ? (
        <p className="text-sm text-neutral-400">{t('common.loading')}</p>
      ) : endpoints.state.status === 'error' ? (
        <p className="text-sm text-red-600">
          {t('common.error')}: {endpoints.state.detail}
        </p>
      ) : rows.length === 0 ? (
        <p className="text-sm text-neutral-500">{t('infra.none.endpoints')}</p>
      ) : (
        <EntityTable
          columns={columns}
          rows={rows}
          getKey={(e) => e.id}
          gridClass="grid-cols-[minmax(0,1fr)_minmax(0,5rem)_minmax(0,1fr)_minmax(0,1fr)_minmax(0,10rem)]"
        />
      )}
    </Stack>
  );
}

// --- Формы (определение) ----------------------------------------------------

type EditableFormField = { name: string; type: string; required: boolean; label: string; validation: string };
const FIELD_TYPES = ['text', 'email', 'password', 'number', 'select', 'checkbox', 'textarea', 'custom'];

function FormDefinitionEditor({ siteId, form, targets }: { siteId: string; form: AdminForm; targets: string[] }) {
  const { api, t } = useAdmin();
  const def = form.definition ?? {};
  const defFields = (def.fields ?? []) as Array<Record<string, unknown>>;

  const [name, setName] = useState(form.name ?? '');
  const [target, setTarget] = useState(
    String(
      typeof def.submit === 'object' && def.submit !== null ? def.submit.target ?? def.submit.endpoint ?? '' : '',
    ),
  );
  const [fields, setFields] = useState<EditableFormField[]>(
    defFields.map((f, i) => ({
      name: String(f.name ?? ''),
      type: String(f.type ?? 'text'),
      required: Boolean(f.required),
      label: String(f.label ?? ''),
      validation: typeof f.validation === 'object' && f.validation !== null ? JSON.stringify(f.validation) : '',
    })),
  );
  const [error, setError] = useState('');
  const [saving, setSaving] = useState(false);

  const setField = (i: number, patch: Partial<EditableFormField>) =>
    setFields((fs) => fs.map((f, idx) => (idx === i ? { ...f, ...patch } : f)));

  const save = async () => {
    setError('');
    if (!target) {
      setError('Выберите submit.target');
      return;
    }
    const parsedFields: Array<Record<string, unknown>> = [];
    for (const f of fields) {
      if (f.name.trim() === '') continue;
      const row: Record<string, unknown> = { name: f.name, type: f.type };
      if (f.required) row.required = true;
      if (f.label) row.label = f.label;
      if (f.validation.trim()) {
        try {
          row.validation = JSON.parse(f.validation);
        } catch (e) {
          setError(e instanceof Error ? e.message : String(e));
          return;
        }
      }
      parsedFields.push(row);
    }
    const definition = {
      id: form.id,
      fields: parsedFields,
      submit: { target },
    };
    setSaving(true);
    const res = await runOperation(api, 'updateForm', { siteId, formId: form.id, patch: { name, definition } }, t);
    setSaving(false);
    if (!res.ok) {
      setError(res.detail);
      return;
    }
    window.alert(t('infra.save') + ' ✓');
  };

  return (
    <div className="rounded border border-neutral-200 bg-white p-4">
      <div className="mb-3 flex items-center justify-between">
        <h3 className="text-sm font-medium text-neutral-900">{form.id}</h3>
        <button
          type="button"
          onClick={save}
          disabled={saving}
          className="rounded bg-blue-600 px-3 py-1.5 text-sm text-white hover:bg-blue-700 disabled:opacity-60"
        >
          {saving ? t('infra.save') + '…' : t('infra.save')}
        </button>
      </div>
      <Inline gap={3} wrap>
        <Field label="Название">
          <input className={inputClass} value={name} onChange={(e) => setName(e.target.value)} />
        </Field>
        <Field label="submit.target" required>
          <select className={inputClass} value={target} onChange={(e) => setTarget(e.target.value)}>
            <option value="">—</option>
            {targets.map((t2) => (
              <option key={t2} value={t2}>
                {t2}
              </option>
            ))}
          </select>
        </Field>
      </Inline>

      <div className="mt-4">
        <div className="mb-1 text-xs font-medium text-neutral-500">Формы</div>
        {fields.length === 0 ? (
          <p className="text-sm text-neutral-500">Полей нет</p>
        ) : (
          <div className="flex flex-col gap-2">
            {fields.map((f, i) => (
              <Inline key={i} gap={2} wrap>
                <Field label="name">
                  <input className={inputClass} value={f.name} onChange={(e) => setField(i, { name: e.target.value })} />
                </Field>
                <Field label="type">
                  <select className={inputClass} value={f.type} onChange={(e) => setField(i, { type: e.target.value })}>
                    {FIELD_TYPES.map((ft) => (
                      <option key={ft} value={ft}>
                        {ft}
                      </option>
                    ))}
                  </select>
                </Field>
                <Field label="label">
                  <input className={inputClass} value={f.label} onChange={(e) => setField(i, { label: e.target.value })} />
                </Field>
                <Field label="validation (JSON)">
                  <input
                    className={inputClass}
                    value={f.validation}
                    placeholder='{"patterns":["^[a-z]+$"]}'
                    onChange={(e) => setField(i, { validation: e.target.value })}
                  />
                </Field>
                <Field label="required">
                  <input type="checkbox" checked={f.required} onChange={(e) => setField(i, { required: e.target.checked })} />
                </Field>
                <button type="button" onClick={() => setFields((fs) => fs.filter((_, idx) => idx !== i))} className="text-sm text-red-600">
                  ×
                </button>
              </Inline>
            ))}
          </div>
        )}
        <button
          type="button"
          onClick={() => setFields((fs) => [...fs, { name: '', type: 'text', required: false, label: '', validation: '' }])}
          className="mt-2 text-sm text-blue-700 hover:underline"
        >
          + добавить поле
        </button>
      </div>

      {error && <p className="mt-2 text-sm text-red-600">{error}</p>}
      <p className="mt-3 text-xs text-neutral-400">{t('infra.form.fieldsReferences')}</p>
    </div>
  );
}

function FormsTab() {
  const { siteId = '' } = useParams();
  const { t } = useAdmin();
  const forms = useOperation<AdminForm[]>(
    'listForms',
    { siteId },
    (d) => (Array.isArray(d) ? (d as AdminForm[]) : []),
  );
  const ops = useOperation<AdminOperation[]>(
    'listOperations',
    { siteId },
    (d) => (Array.isArray(d) ? (d as AdminOperation[]) : []),
  );
  const endpoints = useOperation<AdminEndpoint[]>(
    'listEndpoints',
    { siteId },
    (d) => (Array.isArray(d) ? (d as AdminEndpoint[]) : []),
  );
  const [selected, setSelected] = useState<string | null>(null);

  const allForms = forms.state.status === 'success' ? forms.state.data : [];
  const form = useMemo(() => allForms.find((f) => f.id === selected) ?? null, [allForms, selected]);

  const targets = useMemo(() => {
    const list: string[] = [];
    if (endpoints.state.status === 'success') list.push(...endpoints.state.data.map((e) => `endpoint.${e.id}`));
    if (ops.state.status === 'success') list.push(...ops.state.data.map((o) => `operation.${o.id}`));
    return list.sort();
  }, [endpoints.state, ops.state]);

  return (
    <Stack gap={3}>
      {forms.state.status === 'loading' ? (
        <p className="text-sm text-neutral-400">{t('common.loading')}</p>
      ) : forms.state.status === 'error' ? (
        <p className="text-sm text-red-600">
          {t('common.error')}: {forms.state.detail}
        </p>
      ) : allForms.length === 0 ? (
        <p className="text-sm text-neutral-500">{t('infra.none.operations')}</p>
      ) : (
        <div className="grid grid-cols-[minmax(0,16rem)_minmax(0,1fr)] items-start gap-3">
          <div className="flex w-64 flex-col gap-1">
            {allForms.map((f) => (
              <button
                key={f.id}
                type="button"
                aria-pressed={selected === f.id}
                onClick={() => setSelected(f.id === selected ? null : f.id)}
                className={`rounded px-2 py-1.5 text-left text-sm ${
                  selected === f.id ? 'bg-blue-50 font-medium text-blue-800' : 'text-neutral-600 hover:bg-neutral-100'
                }`}
              >
                {f.id}
              </button>
            ))}
          </div>
          {form ? <FormDefinitionEditor siteId={siteId} form={form} targets={targets} /> : null}
        </div>
      )}
    </Stack>
  );
}

export default InfraSection;