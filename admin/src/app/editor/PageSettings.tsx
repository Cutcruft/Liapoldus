import { useId } from 'react';
import { useAdmin } from '../admin-context';
import { type EditorState, type EditorActions } from './page-store';
import { type BuiltinComponent } from './schemas';

const INPUT =
  'w-full rounded border border-neutral-300 px-2 py-1.5 text-sm focus:border-blue-500 focus:outline-none';
const LABEL = 'block text-sm font-medium text-neutral-700';
const HINT = 'text-xs text-neutral-400';

const ROBOTS_OPTIONS = [
  '',
  'index, follow',
  'noindex, follow',
  'index, nofollow',
  'noindex, nofollow',
] as const;

/**
 * Настройки страницы R10: каркас (layout-section, принимающий лист контента)
 * и per-page head override. Страница перекрывает site-дефолты («База»).
 */
export function PageSettings({
  layoutSectionId,
  head,
  layoutOptions,
  onLayout,
  onHead,
}: {
  layoutSectionId: string;
  head: EditorState['head'];
  layoutOptions: BuiltinComponent[];
  onLayout(sectionId: string): void;
  onHead(patch: Partial<EditorState['head']>): void;
}) {
  const { t } = useAdmin();
  const uid = useId();
  const og = head.og ?? {};
  const meta = head.meta ?? {};

  return (
    <div className="space-y-3 border-b border-neutral-200 p-3">
      <p className="text-xs font-semibold uppercase tracking-wide text-neutral-500">{t('editor.page.settings')}</p>

      <label className={LABEL} htmlFor={`${uid}-layout`}>
        {t('editor.page.layout')}
      </label>
      <select
        id={`${uid}-layout`}
        className={`${INPUT} mt-1`}
        value={layoutSectionId}
        onChange={(e) => onLayout(e.target.value)}
      >
        <option value="">{t('editor.page.layout.default')}</option>
        {layoutOptions.map((c) => (
          <option key={c.type} value={c.type}>
            {c.label}
          </option>
        ))}
      </select>
      <span className={`${HINT} block`}>{t('editor.page.layout.hint')}</span>

      <label className={LABEL} htmlFor={`${uid}-title`}>
        {t('editor.page.title')}
      </label>
      <input
        id={`${uid}-title`}
        className={`${INPUT} mt-1`}
        value={head.title ?? ''}
        placeholder={t('editor.page.title.ph')}
        onChange={(e) => onHead({ title: e.target.value })}
      />

      <label className={LABEL} htmlFor={`${uid}-desc`}>
        {t('editor.page.description')}
      </label>
      <textarea
        id={`${uid}-desc`}
        className={`${INPUT} mt-1 min-h-16`}
        value={head.description ?? ''}
        onChange={(e) => onHead({ description: e.target.value })}
      />

      <label className={LABEL} htmlFor={`${uid}-robots`}>
        {t('editor.page.robots')}
      </label>
      <select
        id={`${uid}-robots`}
        className={`${INPUT} mt-1`}
        value={head.robots ?? ''}
        onChange={(e) => onHead({ robots: e.target.value })}
      >
        {ROBOTS_OPTIONS.map((r) => (
          <option key={r || 'inherit'} value={r}>
            {r || t('editor.page.robots.inherit')}
          </option>
        ))}
      </select>

      <label className={LABEL} htmlFor={`${uid}-canonical`}>
        {t('editor.page.canonical')}
      </label>
      <input
        id={`${uid}-canonical`}
        className={`${INPUT} mt-1`}
        value={head.canonical ?? ''}
        placeholder="https://…"
        onChange={(e) => onHead({ canonical: e.target.value })}
      />

      <label className={LABEL} htmlFor={`${uid}-og`}>
        {t('editor.page.og')}
      </label>
      <textarea
        id={`${uid}-og`}
        className={`${INPUT} mt-1 min-h-24 font-mono text-xs`}
        value={JSON.stringify(og, null, 2)}
        placeholder="{&quot;image&quot;: &quot;https://…&quot;}"
        onChange={(e) => {
          const parsed = parseJsonRecord(e.target.value);
          if (parsed) onHead({ og: parsed });
        }}
      />
      <span className={`${HINT} block`}>{t('editor.page.og.hint')}</span>

      <label className={LABEL} htmlFor={`${uid}-meta`}>
        {t('editor.page.meta')}
      </label>
      <textarea
        id={`${uid}-meta`}
        className={`${INPUT} mt-1 min-h-24 font-mono text-xs`}
        value={JSON.stringify(meta, null, 2)}
        placeholder="{&quot;themeColor&quot;: &quot;#fff&quot;}"
        onChange={(e) => {
          const parsed = parseJsonRecord(e.target.value);
          if (parsed) onHead({ meta: parsed });
        }}
      />
      <span className={`${HINT} block`}>{t('editor.page.meta.hint')}</span>
    </div>
  );
}

/** Парсит JSON-объект строк; возвращает undefined при невалидном/пустом. */
function parseJsonRecord(text: string): Record<string, string> | undefined {
  const trimmed = text.trim();
  if (trimmed === '' || trimmed === '{}') return {};
  try {
    const parsed: unknown = JSON.parse(trimmed);
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return undefined;
    if (Object.values(parsed).some((v) => typeof v !== 'string')) return undefined;
    return parsed as Record<string, string>;
  } catch {
    return undefined;
  }
}