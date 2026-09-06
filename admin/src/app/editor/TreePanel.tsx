import { useState } from 'react';
import { useSelector } from '@liapoldus/ui-runtime';
import type { ComponentNode, Translate } from '../../runtime';
import { findNode } from './tree-utils';
import type { BuiltinComponent } from './schemas';
import type { EditorStore, EditorActions } from './page-store';
import { ToolButton } from './controls';

interface TreeRowProps {
  node: ComponentNode;
  depth: number;
  components: readonly BuiltinComponent[];
  selectionId?: string;
  addingId?: string;
  isRoot: boolean;
  onSelect: (id: string) => void;
  onToggleAdd: (id: string) => void;
  actions: EditorActions;
  t: Translate;
}

function TreeRow({
  node,
  depth,
  components,
  selectionId,
  addingId,
  isRoot,
  onSelect,
  onToggleAdd,
  actions,
  t,
}: TreeRowProps) {
  const selected = selectionId === node.id;
  const summary =
    (node.props?.['label'] as string) ?? (node.props?.['text'] as string) ?? node.type;
  const isContainer = components.some((c) => c.type === node.type && c.container);

  return (
    <div>
      <div
        role="button"
        tabIndex={0}
        aria-selected={selected}
        onClick={() => onSelect(node.id)}
        onKeyDown={(e) => e.key === 'Enter' && onSelect(node.id)}
        className={`flex items-center gap-1 rounded px-1 py-0.5 text-sm ${
          selected ? 'bg-blue-50 text-blue-700 ring-1 ring-blue-300' : 'text-neutral-700 hover:bg-neutral-100'
        }`}
        style={{ paddingLeft: `${8 + depth * 16}px` }}
      >
        <span className="font-medium">{node.type}</span>
        {summary !== node.type && <span className="truncate text-neutral-400">· {summary}</span>}
        <span className="ml-auto flex gap-1">
          {isContainer && (
            <ToolButton title={t('editor.addChild')} onClick={() => onToggleAdd(node.id)}>
              +
            </ToolButton>
          )}
          {!isRoot && (
            <>
              <ToolButton title={t('editor.up')} onClick={() => actions.move(node.id, 'up')}>
                ↑
              </ToolButton>
              <ToolButton title={t('editor.down')} onClick={() => actions.move(node.id, 'down')}>
                ↓
              </ToolButton>
              <ToolButton title={t('editor.delete')} onClick={() => actions.remove(node.id)}>
                ✕
              </ToolButton>
            </>
          )}
        </span>
      </div>

      {addingId === node.id && (
        <div className="ml-6 mt-0.5 flex flex-wrap gap-1 py-0.5">
          {components.map((c) => (
            <button
              key={c.type}
              type="button"
              className="rounded border border-blue-200 bg-blue-50 px-1.5 py-0.5 text-xs text-blue-700 hover:bg-blue-100"
              onClick={() => {
                actions.insert(node.id, c.type, c.schema);
                onToggleAdd(node.id);
              }}
            >
              + {c.label}
            </button>
          ))}
        </div>
      )}

      {(node.children ?? []).map((child) => (
        <TreeRow
          key={child.id}
          node={child}
          depth={depth + 1}
          components={components}
          selectionId={selectionId}
          addingId={addingId}
          isRoot={false}
          onSelect={onSelect}
          onToggleAdd={onToggleAdd}
          actions={actions}
          t={t}
        />
      ))}
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
  const tree = useSelector(store, (s) => s.tree);
  const selectionId = useSelector(store, (s) => s.selectionId);
  const [addingId, setAddingId] = useState<string | undefined>(undefined);
  const toggleAdd = (id: string) => setAddingId((cur) => (cur === id ? undefined : id));

  if (!tree) return <p className="text-xs text-neutral-400">{t('editor.selectHint')}</p>;

  return (
    <div className="flex flex-col gap-1">
      <TreeRow
        node={tree}
        depth={0}
        components={components}
        isRoot
        selectionId={selectionId}
        addingId={addingId}
        onSelect={actions.select}
        onToggleAdd={toggleAdd}
        actions={actions}
        t={t}
      />
    </div>
  );
}

export function findSelected(tree: ComponentNode | undefined, selectionId?: string): ComponentNode | undefined {
  return tree && selectionId ? findNode(tree, selectionId) : undefined;
}