import { useEffect, useState } from 'react';
import { useParams, useSearchParams } from 'react-router-dom';
import { Inline, Stack } from '@liapoldus/ui-kit';
import { runOperation, type AdminComponent, type ComponentHistoryEntry, type ComponentRegistry, type ComponentUsage, type RegistryComponent } from '../../runtime';
import { useAdmin } from '../admin-context';
import { useOperation } from '../use-operation';
import { EntityTable } from './EntityTable';
import { Field } from './Field';
import { TsxEditor } from './TsxEditor';
import { inferProps, inferComponentName } from './schema-utils';
import { validateImportPolicy, validateTsx, siteComponentImports, type TsxDiagnostic, type ImportPolicyRegistryEntry } from './tsx-validate';

const INPUT_CLASS =
  'rounded border border-neutral-300 px-2 py-1.5 text-sm focus:border-blue-500 focus:outline-none focus:outline-none';

/** Дефолтный TSX нового компонента. */
const DEFAULT_SNIPPET = [
  'export interface Props {',
  '  title: string',
  '}',
  '',
  'export default function Component({ title }: Props) {',
  '  return (',
  '    <section>',
  '      <h1>{title}</h1>',
  '    </section>',
  '  )',
  '}',
].join('\n');

function coerceRegistry(d: unknown): ComponentRegistry {
  if (Array.isArray(d)) return { components: d as RegistryComponent[] };
  if (d && typeof d === 'object' && Array.isArray((d as { components?: unknown }).components)) {
    return d as ComponentRegistry;
  }
  return { components: [] };
}

function coerceComponent(d: unknown): AdminComponent | null {
  return d && typeof d === 'object' && 'source' in (d as object) ? (d as AdminComponent) : null;
}

function coerceHistory(d: unknown): ComponentHistoryEntry[] {
  return Array.isArray(d) ? (d as ComponentHistoryEntry[]) : [];
}

function coerceUsage(d: unknown, componentId: string): ComponentUsage {
  if (d && typeof d === 'object' && Array.isArray((d as { pages?: unknown }).pages)) {
    return d as ComponentUsage;
  }
  return { componentId, pages: [] };
}

/**
 * Реестр компонентов (R5): список определений сайта с версией (dev-HEAD),
 * признаком «сохранён/не сохранён» и числом использований в деревьях страниц.
 * Открытие ведёт в `?view=editor&section=components&componentId=...`.
 */
export function ComponentsRegistry() {
  const { siteId = '' } = useParams();
  const [, setSearchParams] = useSearchParams();
  const { api, t } = useAdmin();
  const list = useOperation<ComponentRegistry>('componentRegistry', { siteId }, coerceRegistry);

  const [formOpen, setFormOpen] = useState(false);
  const [name, setName] = useState('');
  const [newSection, setNewSection] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [formError, setFormError] = useState('');

  const create = async (e: React.FormEvent) => {
    e.preventDefault();
    const clean = name.trim();
    if (!clean) {
      setFormError(t('common.required'));
      return;
    }
    setSubmitting(true);
    setFormError('');
    const res = await runOperation(
      api,
      'createComponent',
      { siteId, name: clean, kind: 'component', isSection: newSection, source: DEFAULT_SNIPPET, schema: {} },
      t,
    );
    setSubmitting(false);
    if (!res.ok) {
      setFormError(res.detail);
      return;
    }
    const created = res.data && typeof res.data === 'object' && 'id' in (res.data as object)
      ? String((res.data as { id: unknown }).id)
      : '';
    setName('');
    setFormOpen(false);
    list.reload();
    if (created) {
      setSearchParams((p) => {
        p.set('view', 'editor');
        p.set('section', 'components');
        p.set('componentId', created);
        return p;
      });
    }
  };

  const open = (c: RegistryComponent) => {
    setSearchParams((p) => {
      p.set('view', 'editor');
      p.set('section', 'components');
      p.set('componentId', c.id);
      return p;
    });
  };

  const reg: ComponentRegistry = list.state.status === 'success' ? list.state.data : { components: [] };

  return (
    <Stack gap={4}>
      <Inline justify="between" align="center">
        <h1 className="text-xl font-medium">{t('components.title')}</h1>
        <button
          type="button"
          onClick={() => {
            setFormOpen((v) => !v);
            setFormError('');
          }}
          className="rounded bg-blue-600 px-3 py-1.5 text-sm text-white hover:bg-blue-700"
        >
          {t('components.new')}
        </button>
      </Inline>

      {reg.devSha && (
        <p className="text-xs text-neutral-400">
          dev: <code className="rounded bg-neutral-100 px-1 py-0.5">{reg.devSha.slice(0, 10)}</code>
        </p>
      )}

      {formOpen && (
        <form onSubmit={create} className="rounded border border-neutral-200 bg-neutral-50 p-4">
          <Inline gap={3} align="end">
            <Field label={t('components.name')} required>
              <input
                className={INPUT_CLASS}
                value={name}
                placeholder={t('components.name.placeholder')}
                onChange={(e) => setName(e.target.value)}
              />
            </Field>
            <Field label={t('components.kind')}>
              <select
                className={INPUT_CLASS}
                value={newSection ? 'section' : 'primitive'}
                onChange={(e) => setNewSection(e.target.value === 'section')}
                aria-label={t('components.kind')}
              >
                <option value="primitive">{t('components.isPrimitive')}</option>
                <option value="section">{t('components.isSection')}</option>
              </select>
            </Field>
            <Inline gap={2}>
              <button
                type="submit"
                disabled={submitting}
                className="rounded bg-blue-600 px-3 py-1.5 text-sm text-white hover:bg-blue-700 disabled:opacity-50"
              >
                {t('components.create')}
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
          {formError && <p className="mt-2 text-sm text-red-600">{formError}</p>}
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
        (reg.components.length === 0 ? (
          <p className="text-sm text-neutral-400">{t('components.none')}</p>
        ) : (
          <EntityTable<RegistryComponent>
            gridClass="grid-cols-[minmax(0,2fr)_minmax(0,6rem)_minmax(0,6.5rem)_minmax(0,8rem)_minmax(0,10rem)]"
            getKey={(c) => c.id}
            columns={[
              {
                key: 'name',
                label: t('components.name'),
                render: (c) => (
                  <span className="inline-flex min-w-0 items-center gap-2">
                    <button
                      type="button"
                      onClick={() => open(c)}
                      className="truncate text-left font-medium text-neutral-900 hover:text-blue-600"
                    >
                      {c.name}
                    </button>
                    <span
                      className={`shrink-0 rounded px-1.5 py-0.5 text-[10px] uppercase tracking-wide ${
                        c.isSection ? 'bg-violet-50 text-violet-700' : 'bg-neutral-100 text-neutral-500'
                      }`}
                    >
                      {t(c.isSection ? 'components.badge.section' : 'components.badge.primitive')}
                    </span>
                  </span>
                ),
              },
              { key: 'kind', label: t('components.kind'), render: (c) => <code className="text-xs">{c.kind}</code> },
              {
                key: 'usage',
                label: t('components.version'),
                render: (c) => (
                  <span className={c.usageCount === 0 ? 'text-neutral-400' : 'text-neutral-700'}>
                    {t(c.usageCount === 0 ? 'components.usage.none' : 'components.usage.pages', {
                      n: String(c.usageCount),
                    })}
                  </span>
                ),
              },
              {
                key: 'state',
                label: '',
                render: (c) => (
                  <span
                    className={`inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs ${
                      c.committed ? 'bg-green-50 text-green-700' : 'bg-amber-50 text-amber-700'
                    }`}
                  >
                    <span
                      className={`h-1.5 w-1.5 rounded-full ${c.committed ? 'bg-green-500' : 'bg-amber-500'}`}
                      aria-hidden="true"
                    />
                    {t(c.committed ? 'components.committed' : 'components.dirty')}
                  </span>
                ),
              },
              {
                key: 'updated',
                label: t('components.updatedAt'),
                render: (c) => <span className="text-neutral-500">{shortDate(c.updatedAt)}</span>,
              },
            ]}
            rows={reg.components}
          />
        ))}
    </Stack>
  );
}

function shortDate(iso: string | undefined): string {
  if (!iso) return '—';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleString('ru-RU', { day: '2-digit', month: '2-digit', year: '2-digit', hour: '2-digit', minute: '2-digit' });
}

export function ComponentsSection() {
  const [searchParams] = useSearchParams();
  const componentId = searchParams.get('componentId');
  if (componentId) {
    return <ComponentDetail key={componentId} componentId={componentId} />;
  }
  return <ComponentsRegistry />;
}

/** Исторический исходник (viewSha != null): read-only редактор поверх `?sha=` запроса. */
function SourceView({ siteId, componentId, sha, onBack }: { siteId: string; componentId: string; sha: string; onBack: () => void }) {
  const { t } = useAdmin();
  const history = useOperation<AdminComponent | null>(
    'getComponent',
    { siteId, componentId, query: { sha } },
    coerceComponent,
  );
  return (
    <Stack gap={2}>
      <Inline justify="between" align="center">
        <p className="text-sm text-neutral-600">
          {t('components.history.readonly', { short: sha.slice(0, 10) })}
        </p>
        <button
          type="button"
          onClick={onBack}
          className="rounded border border-neutral-300 px-2 py-1 text-xs text-neutral-600 hover:border-blue-300 hover:text-blue-600"
        >
          {t('components.backToCurrent')}
        </button>
      </Inline>
      {history.state.status === 'loading' && <p className="text-sm text-neutral-400">{t('common.loading')}</p>}
      {history.state.status === 'success' && history.state.data && (
        <TsxEditor value={history.state.data.source} onChange={() => {}} readOnly />
      )}
    </Stack>
  );
}

export function ComponentDetail({ componentId }: { componentId: string }) {
  const { siteId = '' } = useParams();
  const [, setSearchParams] = useSearchParams();
  const { api, t } = useAdmin();

  const current = useOperation<AdminComponent | null>('getComponent', { siteId, componentId }, coerceComponent);
  const history = useOperation<ComponentHistoryEntry[]>('componentHistory', { siteId, componentId }, coerceHistory);
  const usage = useOperation<ComponentUsage>('componentUsage', { siteId, componentId }, (d) => coerceUsage(d, componentId));
  const registry = useOperation<ComponentRegistry>('componentRegistry', { siteId }, coerceRegistry);

  const [source, setSource] = useState('');
  const [name, setName] = useState('');
  const [isSection, setIsSection] = useState(false);
  const [acceptsPageContent, setAcceptsPageContent] = useState(false);
  const [allowedIds, setAllowedIds] = useState<string[]>([]);
  const [viewSha, setViewSha] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState('');
  const [message, setMessage] = useState('');
  const [committing, setCommitting] = useState(false);
  const [commitError, setCommitError] = useState('');
  const [diag, setDiag] = useState<TsxDiagnostic>({ ok: true });

  const comp = current.state.status === 'success' ? current.state.data : null;

  // Синхронизация редактора с загруженным определением.
  useEffect(() => {
    if (current.state.status === 'success' && current.state.data) {
      setSource(current.state.data.source);
      setName(current.state.data.name);
      setIsSection(!!current.state.data.isSection);
      setAcceptsPageContent(!!current.state.data.acceptsPageContent);
      setAllowedIds(current.state.data.allowedPrimitiveIds ?? []);
    }
  }, [current.state.status, componentId]);

  const dirty =
    !viewSha &&
    comp != null &&
    (source !== comp.source ||
      name !== comp.name ||
      isSection !== !!comp.isSection ||
      acceptsPageContent !== !!comp.acceptsPageContent ||
      allowedIds.join('|') !== (comp.allowedPrimitiveIds ?? []).join('|'));

  const regEntries: ImportPolicyRegistryEntry[] =
    registry.state.status === 'success' ? registry.state.data.components.map((c) => ({ id: c.id, isSection: c.isSection })) : [];
  const policyErrors = comp ? validateImportPolicy(source, regEntries, isSection, acceptsPageContent, allowedIds) : [];
  const blocked = !diag.ok || policyErrors.length > 0;

  const back = () => {
    setSearchParams((p) => {
      p.delete('componentId');
      return p;
    });
  };

  const save = async () => {
    if (!comp) return;
    setSaving(true);
    setSaveError('');
    const res = await runOperation(
      api,
      'updateComponent',
      {
        siteId,
        componentId,
        name,
        kind: comp.kind,
        isSection,
        acceptsPageContent,
        allowedPrimitiveIds: allowedIds,
        source,
        schema: comp.schema ?? {},
        metadata: comp.metadata ?? {},
      },
      t,
    );
    setSaving(false);
    if (!res.ok) {
      setSaveError(res.detail);
      return;
    }
    current.reload();
  };

  const commit = async () => {
    setCommitting(true);
    setCommitError('');
    const res = await runOperation(api, 'componentCommit', { siteId, componentId, message: message.trim() }, t);
    setCommitting(false);
    if (!res.ok) {
      setCommitError(res.detail);
      return;
    }
    setMessage('');
    current.reload();
    history.reload();
  };

  // ⌘S / Ctrl+S — сохранить.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 's') {
        e.preventDefault();
        if (!viewSha && comp && dirty) void save();
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [viewSha, comp, dirty]);

  const inferred = inferProps(comp?.source ?? '');
  const detectedName = inferComponentName(comp?.source ?? '');
  const historyEntries: ComponentHistoryEntry[] = history.state.status === 'success' ? history.state.data : [];
  const usageData: ComponentUsage = usage.state.status === 'success' ? usage.state.data : { componentId, pages: [] };

  return (
    <Stack gap={4}>
      <Inline justify="between" align="center">
        <Inline gap={3} align="center">
          <button
            type="button"
            onClick={back}
            className="rounded border border-neutral-300 px-2 py-1 text-xs text-neutral-600 hover:border-blue-300 hover:text-blue-600"
          >
            {`← ${t('components.back')}`}
          </button>
          <h1 className="text-xl font-medium">{name || t('components.title')}</h1>
          {viewSha ? (
            <span className="rounded bg-blue-50 px-2 py-0.5 text-xs text-blue-700">
              {t('components.history.readonly', { short: viewSha.slice(0, 10) })}
            </span>
          ) : comp ? (
            <span
              className={`inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs ${
                dirty ? 'bg-amber-50 text-amber-700' : 'bg-green-50 text-green-700'
              }`}
            >
              <span className={`h-1.5 w-1.5 rounded-full ${dirty ? 'bg-amber-500' : 'bg-green-500'}`} aria-hidden="true" />
              {t(dirty ? 'components.dirty' : 'components.committed')}
            </span>
          ) : null}
        </Inline>
        <Inline gap={2}>
          {dirty && (
            <button
              type="button"
              onClick={() => {
                void save();
              }}
              disabled={saving || blocked}
              className="rounded bg-blue-600 px-3 py-1.5 text-sm text-white hover:bg-blue-700 disabled:opacity-50"
            >
              {t('components.save')}
            </button>
          )}
        </Inline>
      </Inline>

      {current.state.status === 'loading' && <p className="text-sm text-neutral-400">{t('common.loading')}</p>}
      {current.state.status === 'error' && (
        <p className="text-sm text-red-600">
          {t('common.error')}: {current.state.detail}
        </p>
      )}

      {comp && (
        <div className="grid min-h-0 grid-cols-1 gap-4 lg:grid-cols-[minmax(0,1fr)_18rem]">
          <Stack gap={3} className="min-w-0">
            {viewSha ? (
              <SourceView siteId={siteId} componentId={componentId} sha={viewSha} onBack={() => setViewSha(null)} />
            ) : (
              <>
                <TsxEditor value={source} onChange={setSource} onDiagnostic={setDiag} />
                <Inline gap={3} align="center">
                  <span
                    className={`text-xs ${diag.ok ? 'text-green-700' : 'text-red-600'}`}
                    data-testid="tsx-diagnostic"
                  >
                    {diag.ok
                      ? t('components.validation.ok')
                      : t('components.validation.error', {
                          line: String(diag.line ?? 1),
                          message: diag.message ?? '',
                        })}
                  </span>
                  <span
                    className={`hidden align-middle text-xs sm:inline ${policyErrors.length === 0 ? 'text-green-700' : 'text-red-600'}`}
                    data-testid="import-policy-diagnostic"
                  >
                    {policyErrors.length === 0
                      ? t('components.policy.ok')
                      : t('components.policy.error')}
                  </span>
                  {saveError && <span className="text-xs text-red-600">{saveError}</span>}
                </Inline>
                {policyErrors.length > 0 && (
                  <ul className="space-y-1 rounded border border-red-200 bg-red-50 p-2" data-testid="policy-errors">
                    {policyErrors.map((msg) => (
                      <li key={msg} className="text-xs text-red-700">
                        {msg}
                      </li>
                    ))}
                  </ul>
                )}
                {isSection && (
                  <p className="text-xs text-neutral-400">
                    {t('components.imports')}:{' '}
                    <code className="text-neutral-600">
                      {siteComponentImports(source).length > 0
                        ? siteComponentImports(source).map((i) => `@site/components/${i}`).join(', ')
                        : t('components.imports.none')}
                    </code>
                  </p>
                )}
              </>
            )}
          </Stack>

          <Stack gap={4} className="min-w-0">
            <section className="rounded border border-neutral-200 p-3">
              <h2 className="mb-2 text-sm font-medium text-neutral-800">{t('components.schema.title')}</h2>
              {detectedName && <p className="mb-2 text-xs text-neutral-400">{detectedName}</p>}
              {inferred.length === 0 ? (
                <p className="text-xs text-neutral-400">{t('components.schema.none')}</p>
              ) : (
                <ul className="space-y-1">
                  {inferred.map((prop) => (
                    <li key={prop.name} className="flex items-baseline gap-2 text-xs">
                      <code className={prop.required ? 'font-medium text-neutral-900' : 'text-neutral-600'}>
                        {prop.name}
                      </code>
                      <span className="text-neutral-400">{prop.type}</span>
                      <span className={prop.required ? 'text-orange-600' : 'text-neutral-400'}>
                        {t(prop.required ? 'components.schema.required' : 'components.schema.optional')}
                      </span>
                      {prop.default !== undefined && (
                        <span className="text-neutral-400">= {prop.default}</span>
                      )}
                    </li>
                  ))}
                </ul>
              )}
            </section>

            {!viewSha && (
              <section className="rounded border border-neutral-200 p-3">
                <h2 className="mb-2 text-sm font-medium text-neutral-800">{t('components.policy.title')}</h2>
                <div role="radiogroup" aria-label={t('components.policy.title')} className="grid grid-cols-2 gap-2">
                  {(
                    [
                      { value: false, label: t('components.isPrimitive'), hint: t('components.isPrimitive.hint') },
                      { value: true, label: t('components.isSection'), hint: t('components.isSection.hint') },
                    ] as const
                  ).map((opt) => (
                    <label
                      key={opt.label}
                      className={`cursor-pointer rounded border p-2 text-xs ${
                        isSection === opt.value ? 'border-blue-500 bg-blue-50 text-blue-800' : 'border-neutral-200 text-neutral-600'
                      }`}
                    >
                      <input
                        type="radio"
                        className="sr-only"
                        name="component-role"
                        checked={isSection === opt.value}
                        onChange={() => {
                          setIsSection(opt.value);
                          if (!opt.value) setAcceptsPageContent(false);
                        }}
                      />
                      <span className="block font-medium">{opt.label}</span>
                      <span className="mt-1 block text-neutral-400">{opt.hint}</span>
                    </label>
                  ))}
                </div>

                {isSection && (
                  <Stack gap={3}>
                    <label className="mt-2 flex items-start gap-2 text-xs text-neutral-700">
                      <input
                        type="checkbox"
                        className="mt-0.5"
                        checked={acceptsPageContent}
                        onChange={(e) => setAcceptsPageContent(e.target.checked)}
                      />
                      <span>
                        <span className="block font-medium">{t('components.acceptsPageContent')}</span>
                        <span className="block text-neutral-400">{t('components.acceptsPageContent.hint')}</span>
                      </span>
                    </label>

                    <div>
                      <p className="mb-1 text-xs font-medium text-neutral-800">{t('components.allowlist')}</p>
                      <p className="mb-2 text-[11px] text-neutral-400">{t('components.allowlist.hint')}</p>
                      {regEntries.filter((r) => !r.isSection).length === 0 ? (
                        <p className="text-[11px] text-neutral-400">
                          {t('components.usage.none')}
                        </p>
                      ) : (
                        <ul className="space-y-1">
                          {regEntries
                            .filter((r) => !r.isSection)
                            .map((r) => (
                              <li key={r.id} className="flex items-center gap-2 text-xs text-neutral-700">
                                <input
                                  type="checkbox"
                                  checked={allowedIds.includes(r.id)}
                                  onChange={(e) => {
                                    setAllowedIds((prev) =>
                                      e.target.checked
                                        ? [...new Set([...prev, r.id])]
                                        : prev.filter((id) => id !== r.id),
                                    );
                                  }}
                                />
                                <code>{r.id}</code>
                              </li>
                            ))}
                        </ul>
                      )}
                    </div>
                  </Stack>
                )}
              </section>
            )}

            <section className="rounded border border-neutral-200 p-3">
              <h2 className="mb-2 text-sm font-medium text-neutral-800">{t('components.version')}</h2>
              {usageData.pages.length === 0 ? (
                <p className="text-xs text-neutral-400">{t('components.usage.none')}</p>
              ) : (
                <ul className="space-y-1">
                  {usageData.pages.map((p) => (
                    <li key={p.id} className="flex items-baseline justify-between gap-2 text-xs">
                      <span className="truncate text-neutral-700">{p.name}</span>
                      <span className="text-neutral-400">{p.count}</span>
                    </li>
                  ))}
                </ul>
              )}
            </section>

            <section className="rounded border border-neutral-200 p-3">
              <h2 className="mb-2 text-sm font-medium text-neutral-800">{t('components.history')}</h2>
              {history.state.status === 'success' && historyEntries.length === 0 && (
                <p className="text-xs text-neutral-400">{t('components.history.none')}</p>
              )}
              {history.state.status === 'error' && (
                <p className="text-xs text-red-600">{t('common.error')}: {history.state.detail}</p>
              )}
              <ul className="space-y-1">
                {historyEntries.map((h) => (
                  <li key={h.sha} className="flex items-baseline justify-between gap-2 text-xs">
                    <span className="truncate text-neutral-700">
                      {h.message} <span className="text-neutral-400">({h.sha.slice(0, 7)})</span>
                    </span>
                    <button
                      type="button"
                      onClick={() => setViewSha(h.sha)}
                      className="shrink-0 rounded border border-neutral-300 px-1.5 py-0.5 text-neutral-600 hover:border-blue-300 hover:text-blue-600"
                    >
                      {t('components.history.restore')}
                    </button>
                  </li>
                ))}
              </ul>
            </section>

            {!viewSha && (
              <section className="rounded border border-neutral-200 p-3">
                <h2 className="mb-2 text-sm font-medium text-neutral-800">{t('components.commit')}</h2>
                <Inline gap={2}>
                  <input
                    className={INPUT_CLASS}
                    value={message}
                    placeholder={t('components.commit.placeholder')}
                    onChange={(e) => setMessage(e.target.value)}
                    aria-label={t('components.commit.placeholder')}
                  />
                  <button
                    type="button"
                    onClick={() => {
                      void commit();
                    }}
                    disabled={committing || message.trim() === ''}
                    className="rounded border border-neutral-300 px-3 py-1.5 text-sm text-neutral-700 hover:border-blue-300 hover:text-blue-600 disabled:opacity-50"
                  >
                    {t('components.commit')}
                  </button>
                </Inline>
                {commitError && <p className="mt-2 text-xs text-red-600">{commitError}</p>}
              </section>
            )}
          </Stack>
        </div>
      )}
    </Stack>
  );
}