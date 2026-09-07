import { useSelector } from '@liapoldus/ui-runtime';
import type { ElementNode, Translate } from '../../runtime';
import type { EditorStore } from './page-store';

const SIZE: Record<string, number> = { sm: 12, md: 16, lg: 22, xl: 30 };

function propValue(el: ElementNode, field: string): unknown | undefined {
  const p = el.props[field];
  if (!p || p.kind !== 'literal') return undefined;
  return p.value;
}

function bindingLabel(el: ElementNode, field: string): string | undefined {
  const p = el.props[field];
  if (!p || p.kind !== 'binding') return undefined;
  const s = p.source;
  switch (s.kind) {
    case 'content':
      return `{{content:${s.contentId}.${s.field}}}`;
    case 'form':
      return `{{form:${s.formId}}}`;
    case 'operation':
      return `{{operation:${s.operationId}}}`;
    case 'query':
      return `{{query:${s.param}}}`;
    case 'routeGroup':
      return `{{route.$${s.index}}}`;
  }
}

function RenderElement({ el, t }: { el: ElementNode; t: Translate }) {
  const type = el.componentId;

  switch (type) {
    case 'Container': {
      const layout = String(propValue(el, 'layout') ?? 'stack');
      return (
        <div
          data-node={type}
          className="rounded border border-dashed border-blue-200 p-2"
          style={{
            display: 'flex',
            flexDirection: layout === 'row' ? 'row' : 'column',
            flexWrap: layout === 'row' ? 'wrap' : undefined,
            gap: Number(propValue(el, 'gap') ?? 0),
          }}
        >
          {t('editor.container')}
        </div>
      );
    }
    case 'Text': {
      const label = bindingLabel(el, 'text');
      if (label) {
        return (
          <span data-node={type} className="whitespace-pre-wrap font-mono text-xs text-blue-600">
            {label}
          </span>
        );
      }
      const html = String(propValue(el, 'text') ?? '');
      return (
        <span
          data-node={type}
          // Дизайн-превью: html рендерится как есть (без санитизации).
          dangerouslySetInnerHTML={{ __html: html }}
          className="whitespace-pre-wrap"
          style={{
            fontSize: SIZE[String(propValue(el, 'size'))] ?? 16,
            textAlign: String(propValue(el, 'align') ?? 'left') as React.CSSProperties['textAlign'],
            color: String(propValue(el, 'color') ?? ''),
          }}
        />
      );
    }
    case 'Image': {
      const assetId = String(propValue(el, 'assetId') ?? '');
      if (!assetId) {
        return (
          <div
            data-node={type}
            className="flex items-center justify-center rounded border border-dashed border-neutral-300 py-6 text-xs text-neutral-400"
          >
            {t('editor.selectHint')}
          </div>
        );
      }
      const src = /^[\w-]+:/.test(assetId) || assetId.startsWith('/') ? assetId : `/api/assets/${assetId}/file`;
      return (
        <img
          data-node={type}
          src={src}
          alt={String(propValue(el, 'alt') ?? '')}
          style={{ width: Number(propValue(el, 'width') ?? 320) }}
        />
      );
    }
    case 'Button': {
      const label = bindingLabel(el, 'label');
      return (
        <button
          data-node={type}
          type="button"
          className="rounded bg-blue-600 px-3 py-1.5 text-sm text-white"
        >
          {label ?? String(propValue(el, 'label') ?? '')}
        </button>
      );
    }
    default:
      return (
        <div
          data-node="Unknown"
          className="rounded border border-neutral-300 px-2 py-1 text-xs text-neutral-400"
        >
          {type}
        </div>
      );
  }
}

/** Канвас-превью в design-mode: рендерит текущий (в т.ч. несохранённый) лист элементов. */
export function PreviewPane({ store, t }: { store: EditorStore; t: Translate }) {
  const elements = useSelector(store, (s) => s.elements);
  if (elements.length === 0) return <p className="text-sm text-neutral-400">{t('editor.selectHint')}</p>;
  return (
    <div className="flex flex-col gap-2 rounded border border-neutral-200 p-4">
      {elements.map((el) => (
        <RenderElement key={el.id} el={el} t={t} />
      ))}
    </div>
  );
}