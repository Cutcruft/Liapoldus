import { useMemo, useState } from 'react';
import { useParams, useSearchParams } from 'react-router-dom';
import { Inline, Stack } from '@liapoldus/ui-kit';
import { runOperation, type ContentDetail, type Site, type SiteLocales } from '../../runtime';
import { useAdmin } from '../admin-context';
import { useOperation } from '../use-operation';
import { ConfirmButton } from '../components/ConfirmButton';
import { EntityTable } from '../components/EntityTable';
import { clearContentDrafts } from './content-drafts';

/** Заголовок контента: title/heading/name → первое скалярное поле → id. */
export function contentTitle(fields: Record<string, unknown> | undefined, id: string): string {
  for (const key of ['title', 'heading', 'name']) {
    const v = fields?.[key];
    if (typeof v === 'string' && v.trim()) return v.trim();
    if (typeof v === 'number') return String(v);
  }
  const first = fields ? Object.entries(fields)[0] : undefined;
  if (first && String(first[1]).trim()) return `${first[0]}: ${String(first[1])}`;
  return id;
}

function formatDate(value: string | undefined): string {
  if (!value) return '—';
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return '—';
  return d.toLocaleDateString('ru');
}

/**
 * Список контента сайта (R3): фильтры по типу (collection) и языку, колонки
 * заголовок / тип / языки / статус перевода / дата изменения. Открытие item
 * переводит URL в `?mode=content&contentId=...` (редактор контента).
 */
export function ContentList() {
  const { siteId = '' } = useParams();
  const [, setSearchParams] = useSearchParams();
  const { api, t } = useAdmin();

  const site = useOperation<Site | null>('getSite', { siteId }, (d) =>
    typeof d === 'object' && d !== null ? (d as Site) : null,
  );
  const list = useOperation<ContentDetail[]>('listContents', { siteId }, (d) =>
    Array.isArray(d) ? (d as ContentDetail[]) : [],
  );
  const localesInfo = useOperation<SiteLocales | null>('getContentLocales', { siteId }, (d) =>
    typeof d === 'object' && d !== null && 'baseLocale' in d ? (d as SiteLocales) : null,
  );

  const [collectionFilter, setCollectionFilter] = useState<string | null>(null);
  const [languageFilter, setLanguageFilter] = useState<string | null>(null);

  const siteData = site.state.status === 'success' ? site.state.data : null;
  const rows = list.state.status === 'success' ? list.state.data : [];
  const localeInfo = localesInfo.state.status === 'success' ? localesInfo.state.data : null;

  const baseLocale = siteData?.defaultLocale ?? localeInfo?.baseLocale ?? '';

  const collections = useMemo(() => {
    const set = new Set(rows.map((c) => c.collectionId));
    if (collectionFilter && !set.has(collectionFilter)) set.add(collectionFilter);
    return [...set].sort();
  }, [rows, collectionFilter]);

  const languageOptions = useMemo(() => {
    const set = new Set<string>();
    for (const loc of localeInfo?.locales ?? []) {
      if (loc.locale && loc.locale !== baseLocale) set.add(loc.locale);
    }
    for (const c of rows) {
      for (const locale of Object.keys(c.translations ?? {})) {
        if (locale && locale !== baseLocale) set.add(locale);
      }
    }
    return [...set].sort();
  }, [rows, localeInfo, baseLocale]);

  const filtered = useMemo(() => {
    let out = rows;
    if (collectionFilter) out = out.filter((c) => c.collectionId === collectionFilter);
    if (languageFilter) out = out.filter((c) => c.translations?.[languageFilter] !== undefined);
    return out;
  }, [rows, collectionFilter, languageFilter]);

  const remove = async (item: ContentDetail) => {
    const res = await runOperation(api, 'deleteContent', { siteId, contentId: item.id }, t);
    if (res.ok) {
      clearContentDrafts(siteId, item.id);
      list.reload();
    }
  };

  const open = (item: ContentDetail) => {
    setSearchParams({ mode: 'content', contentId: item.id });
  };

  const loading = list.state.status === 'loading' || (site.state.status !== 'error' && site.state.status === 'loading');

  return (
    <Stack gap={4}>
      <Inline justify="between" align="center">
        <h1 className="text-xl font-medium">{t('content.title')}</h1>
        {languageOptions.length > 0 && (
          <label className="flex items-center gap-2 text-sm text-neutral-600">
            {t('content.language')}
            <select
              className="rounded border border-neutral-300 px-2 py-1 text-sm focus:border-blue-500 focus:outline-none"
              value={languageFilter ?? ''}
              onChange={(e) => setLanguageFilter(e.target.value || null)}
            >
              <option value="">{t('content.language.all')}</option>
              {languageOptions.map((locale) => (
                <option key={locale} value={locale}>
                  {locale}
                </option>
              ))}
            </select>
          </label>
        )}
      </Inline>

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

      {loading && <p className="text-sm text-neutral-400">{t('common.loading')}</p>}

      {list.state.status === 'success' && collections.length > 0 && (
        <Inline gap={2} wrap>
          <button
            type="button"
            onClick={() => setCollectionFilter(null)}
            className={`rounded-full border px-3 py-1 text-xs ${
              collectionFilter === null ? 'border-blue-400 bg-blue-50 text-blue-700' : 'border-neutral-300 text-neutral-600'
            }`}
          >
            {t('content.collection.all')}
          </button>
          {collections.map((col) => (
            <button
              key={col}
              type="button"
              onClick={() => setCollectionFilter(col)}
              className={`rounded-full border px-3 py-1 text-xs ${
                collectionFilter === col ? 'border-blue-400 bg-blue-50 text-blue-700' : 'border-neutral-300 text-neutral-600'
              }`}
            >
              {col}
            </button>
          ))}
        </Inline>
      )}

      {list.state.status === 'success' &&
        (filtered.length === 0 ? (
          <p className="text-sm text-neutral-400">{t('content.none')}</p>
        ) : (
          <EntityTable<ContentDetail>
            gridClass="grid-cols-[minmax(0,2fr)_minmax(0,1fr)_minmax(0,9rem)_minmax(0,9rem)_minmax(0,7rem)_minmax(0,9rem)]"
            getKey={(c) => c.id}
            columns={[
              {
                key: 'title',
                label: t('content.title.column'),
                render: (c) => (
                  <button type="button" onClick={() => open(c)} className="truncate text-left font-medium text-neutral-900 hover:text-blue-600">
                    {contentTitle(c.fields, c.id)}
                  </button>
                ),
              },
              { key: 'collection', label: t('content.type.column'), render: (c) => <code className="text-xs">{c.collectionId}</code> },
              {
                key: 'language',
                label: t('content.language.column'),
                render: (c) => {
                  const codes = Object.keys(c.translations ?? {}).filter((l) => l !== baseLocale).sort();
                  if (codes.length === 0) return <span className="text-neutral-400">—</span>;
                  return (
                    <Inline gap={1} wrap>
                      {codes.slice(0, 3).map((l) => (
                        <span key={l} className="rounded bg-neutral-100 px-1.5 py-0.5 text-xs text-neutral-600">
                          {l}
                        </span>
                      ))}
                      {codes.length > 3 && <span className="text-xs text-neutral-400">+{codes.length - 3}</span>}
                    </Inline>
                  );
                },
              },
              {
                key: 'status',
                label: t('content.translations'),
                render: (c) => {
                  const codes = Object.keys(c.translations ?? {}).filter((l) => l !== baseLocale);
                  return codes.length === 0 ? (
                    <span className="text-xs text-neutral-400">{t('content.status.missing')}</span>
                  ) : (
                    <span className="text-xs text-emerald-700">{t('content.status.count', { n: codes.length })}</span>
                  );
                },
              },
              {
                key: 'updatedAt',
                label: t('content.updatedAt'),
                render: (c) => <span className="text-xs text-neutral-500">{formatDate(c.updatedAt)}</span>,
              },
              {
                key: 'actions',
                label: t('common.actions'),
                className: 'text-right',
                render: (c) => (
                  <Inline gap={2} justify="end">
                    <button
                      type="button"
                      onClick={() => open(c)}
                      className="rounded border border-neutral-300 px-2 py-1 text-xs text-neutral-600 hover:border-blue-300 hover:text-blue-600"
                    >
                      {t('content.open')}
                    </button>
                    <ConfirmButton label={t('content.delete.confirm')} onConfirm={() => remove(c)} />
                  </Inline>
                ),
              },
            ]}
            rows={filtered}
          />
        ))}
    </Stack>
  );
}