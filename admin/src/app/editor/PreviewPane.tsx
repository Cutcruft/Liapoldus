import { useSelector } from '@liapoldus/ui-runtime';
import type { ComponentNode, Translate } from '../../runtime';
import type { EditorStore } from './page-store';

const SIZE: Record<string, number> = { sm: 12, md: 16, lg: 22, xl: 30 };

function RenderNode({ node, t }: { node: ComponentNode; t: Translate }) {
  const props = node.props ?? {};
  const children = (node.children ?? []).map((child) => <RenderNode key={child.id} node={child} t={t} />);
  const bound = (field: string) => node.bindings?.[field];

  switch (node.type) {
    case 'Container': {
      const layout = String(props['layout'] ?? 'stack');
      return (
        <div
          data-node={node.type}
          className="rounded border border-dashed border-blue-200 p-2"
          style={{
            display: layout === 'row' ? 'flex' : layout === 'grid' ? 'grid' : 'flex',
            flexDirection: layout === 'row' ? 'row' : 'column',
            gridTemplateColumns: layout === 'grid' ? 'repeat(auto-fit, minmax(0, 1fr))' : undefined,
            gap: Number(props['gap'] ?? 0),
          }}
        >
          {children}
        </div>
      );
    }
    case 'Text': {
      const textBinding = bound('text');
      return (
        <span
          data-node={node.type}
          className="whitespace-pre-wrap"
          style={{
            fontSize: SIZE[String(props['size'])] ?? 16,
            textAlign: String(props['align'] ?? 'left') as React.CSSProperties['textAlign'],
            color: String(props['color'] ?? ''),
          }}
        >
          {textBinding && textBinding.source !== 'literal'
            ? `{{${textBinding.source}:${textBinding.path ?? ''}}}`
            : String(props['text'] ?? '')}
        </span>
      );
    }
    case 'Image': {
      const assetId = String(props['assetId'] ?? '');
      if (!assetId) {
        return (
          <div data-node={node.type} className="flex items-center justify-center rounded border border-dashed border-neutral-300 py-6 text-xs text-neutral-400">
            {t('editor.selectHint')}
          </div>
        );
      }
      const src = /^[\w-]+:/.test(assetId) || assetId.startsWith('/') ? assetId : `/api/assets/${assetId}/file`;
      return (
        <img
          data-node={node.type}
          src={src}
          alt={String(props['alt'] ?? '')}
          style={{ width: Number(props['width'] ?? 320) }}
        />
      );
    }
    case 'Button': {
      const labelBinding = bound('label');
      return (
        <button
          data-node={node.type}
          type="button"
          className="rounded bg-blue-600 px-3 py-1.5 text-sm text-white"
        >
          {labelBinding && labelBinding.source !== 'literal'
            ? `{{${labelBinding.source}:${labelBinding.path ?? ''}}}`
            : String(props['label'] ?? '')}
        </button>
      );
    }
    default:
      return (
        <div data-node="Unknown" className="rounded border border-neutral-300 px-2 py-1 text-xs text-neutral-400">
          {node.type}
        </div>
      );
  }
}

/** Канвас-превью в design-mode: рендерит текущее (в т.ч. несохранённое) дерево. */
export function PreviewPane({ store, t }: { store: EditorStore; t: Translate }) {
  const tree = useSelector(store, (s) => s.tree);
  if (!tree) return <p className="text-sm text-neutral-400">{t('editor.selectHint')}</p>;
  return (
    <div className="rounded border border-neutral-200 p-4">
      <RenderNode node={tree} t={t} />
    </div>
  );
}