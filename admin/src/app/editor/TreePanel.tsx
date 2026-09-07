import { useState } from 'react';
import { useSelector } from '@liapoldus/ui-runtime';
import type { ElementNode, Translate } from '../../runtime';
import { elementSummary, findElement } from './tree-utils';
import type { BuiltinComponent } from './schemas';
import type { EditorStore, EditorActions } from './page-store';
import { ToolButton } from './controls';

interface ElementRowProps {
  el: ElementNode;
  index: number;
  total: number;
  components: readonly BuiltinComponent[];
  selectionId?: string;
  addingId?: string;
  onSelect: (id: string) => void;
  onToggleAdd: (id: string) => void;
  actions: EditorActions;
  t: Translate;
}

function ElementRow({
  el,
  index,
  total,
  components,
  selectionId,
  addingId,
  onSelect,
  onToggleAdd,
  actions,
  t,
}: ElementRowProps) {
  const selected = selectionId === el.id;
  const summary = elementSummary(el);

  return (
    <div>
      <div
        role="button"
        tabIndex={0}
        aria-selected={selected}
        onClick={() => onSelect(el.id)}
        onKeyDown={(e) => e.key === 'Enter' && onSelect(el.id)}
        className={`flex items-center gap-1 rounded px-1 py-0.5 text-sm ${
          selected ? 'bg-blue-50 text-blue-700 ring-1 ring-blue-300' : 'text-neutral-700 hover:bg-neutral-100'
        }`}
      >
        <span className="font-medium">{el.componentId}</span>
        {summary && <span className="truncate text-neutral-400">· {summary}</span>}
        <span className="ml-auto flex gap-1">
          <ToolButton title={t('editor.addChild')} onClick={() => onToggleAdd(el.id)}>
            +
          </ToolButton>
          <ToolButton title={t('editor.up')} disabled={index === 0} onClick={() => actions.move(el.id, 'up')}>
            ↑
          </ToolButton>
          <ToolButton title={t('editor.down')} disabled={index === total - 1} onClick={() => actions.move(el.id, 'down')}>
            ↓
          </ToolButton>
          <ToolButton title={t('editor.delete')} onClick={() => actions.remove(el.id)}>
            ✕
          </ToolButton>
        </span>
      </div>

      {addingId === el.id && (
        <div className="ml-6 mt-0.5 flex flex-wrap gap-1 py-0.5">
          {components.map((c) => (
            <button
              key={c.type}
              type="button"
              className="rounded border border-blue-200 bg-blue-50 px-1.5 py-0.5 text-xs text-blue-700 hover:bg-blue-100"
              onClick={() => {
                actions.insert(c.type, c.schema);
                onToggleAdd(el.id);
              }}
            >
              + {c.label}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

export function TreePanel({
  store,
  actions,
  t,
  components,
}: {
  store: EditorStore;
  actions: EditorActions;
  t: Translate;
  components: readonly BuiltinComponent[];
}) {
  const elements = useSelector(store, (s) => s.elements);
  const selectionId = useSelector(store, (s) => s.selectionId);
  const [addingId, setAddingId] = useState<string | undefined>(undefined);
  const toggleAdd = (id: string) => setAddingId((cur) => (cur === id ? undefined : id));

  if (elements.length === 0) return <p className="text-xs text-neutral-400">{t('editor.selectHint')}</p>;

  return (
    <div className="flex flex-col gap-1">
      {elements.map((el, index) => (
        <ElementRow
          key={el.id}
          el={el}
          index={index}
          total={elements.length}
          components={components}
          selectionId={selectionId}
          addingId={addingId}
          onSelect={actions.select}
          onToggleAdd={toggleAdd}
          actions={actions}
          t={t}
        />
      ))}
    </div>
  );
}

export function findSelected(
  elements: ElementNode[] | undefined,
  selectionId?: string,
): ElementNode | undefined {
  return elements && selectionId ? findElement(elements, selectionId) : undefined;
}

export function elementSummaryForList(el: ElementNode): string | undefined {
  return elementSummary(el);
}