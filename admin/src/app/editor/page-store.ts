import { createSliceStore, type SliceStore } from '@liapoldus/ui-runtime';
import type { BindingSource, ComponentNode } from '../../runtime';
import { findNode, insertChild, makeNode, moveNode, removeNode, replaceNode, uid } from './tree-utils';
import type { JSONSchema } from './schemas';

export interface EditorState {
  status: 'empty' | 'ready' | 'error';
  tree?: ComponentNode;
  selectionId?: string;
  past: ComponentNode[];
  future: ComponentNode[];
  /** Серверная версия последней сохранённой страницы. */
  version: number;
  savedKey: string | null;
  dirty: boolean;
  error?: string;
}

export type EditorStore = SliceStore<EditorState>;

export function createEditorStore(): EditorStore {
  return createSliceStore<EditorState>({
    status: 'empty',
    past: [],
    future: [],
    version: 0,
    savedKey: null,
    dirty: false,
  });
}

const serialize = (node?: ComponentNode): string | null => (node ? JSON.stringify(node) : null);
const HISTORY_CAP = 50;

export interface EditorActions {
  applyLoaded(page: { root: ComponentNode; version: number }): void;
  setLoadError(message: string): void;
  select(id: string): void;
  insert(parentId: string, type: string, schema?: JSONSchema): void;
  remove(id: string): void;
  move(id: string, dir: 'up' | 'down'): void;
  updateProp(id: string, field: string, value: unknown): void;
  setBinding(id: string, field: string, binding: BindingSource): void;
  undo(): void;
  redo(): void;
  applySaved(version: number): void;
}

export function editorActions(store: EditorStore): EditorActions {
  const commit = (nextTree: ComponentNode) => {
    const s = store.getState();
    if (s.tree && serialize(s.tree) === serialize(nextTree)) return;
    const past = [...s.past, s.tree].filter((t): t is ComponentNode => !!t).slice(-HISTORY_CAP);
    store.setState({
      tree: nextTree,
      past,
      future: [],
      dirty: s.savedKey !== serialize(nextTree),
      // selection остаётся, если узел ещё существует, иначе — root
      selectionId: s.selectionId && findNode(nextTree, s.selectionId) ? s.selectionId : nextTree.id,
    });
  };

  return {
    applyLoaded(page) {
      const s = store.getState();
      store.setState({
        status: 'ready',
        tree: page.root,
        version: page.version,
        savedKey: serialize(page.root),
        dirty: false,
        past: [],
        future: [],
        selectionId: page.root.id,
        error: undefined,
      });
      void s;
    },

    setLoadError(message) {
      store.setState({ status: 'error', error: message });
    },

    select(id) {
      const s = store.getState();
      if (s.tree && findNode(s.tree, id)) store.setState({ selectionId: id });
    },

    insert(parentId, type, schema) {
      const s = store.getState();
      const tree = s.tree;
      if (!tree || !findNode(tree, parentId)) return;
      const child = makeNode(type, {}, schema);
      commit(insertChild(tree, parentId, child));
      store.setState({ selectionId: child.id });
    },

    remove(id) {
      const s = store.getState();
      const tree = s.tree;
      if (!tree || tree.id === id) return;
      commit(removeNode(tree, id));
    },

    move(id, dir) {
      const s = store.getState();
      const tree = s.tree;
      if (!tree) return;
      commit(moveNode(tree, id, dir));
    },

    updateProp(id, field, value) {
      const s = store.getState();
      const tree = s.tree;
      if (!tree || !findNode(tree, id)) return;
      const node = findNode(tree, id)!;
      commit(replaceNode(tree, id, { ...node, props: { ...(node.props ?? {}), [field]: value } }));
    },

    setBinding(id, field, binding) {
      const s = store.getState();
      const tree = s.tree;
      if (!tree || !findNode(tree, id)) return;
      const node = findNode(tree, id)!;
      const next = { ...node, bindings: { ...(node.bindings ?? {}), [field]: binding } };
      commit(replaceNode(tree, id, next));
    },

    undo() {
      const s = store.getState();
      if (!s.tree || s.past.length === 0) return;
      const prev = s.past[s.past.length - 1]!;
      store.setState({
        tree: prev,
        past: s.past.slice(0, -1),
        future: [s.tree, ...s.future].slice(0, HISTORY_CAP),
        selectionId: s.selectionId && findNode(prev, s.selectionId) ? s.selectionId : prev.id,
        dirty: s.savedKey !== serialize(prev),
      });
    },

    redo() {
      const s = store.getState();
      if (!s.tree || s.future.length === 0) return;
      const next = s.future[0]!;
      store.setState({
        tree: next,
        past: [...s.past, s.tree].slice(-HISTORY_CAP),
        future: s.future.slice(1),
        selectionId: s.selectionId && findNode(next, s.selectionId) ? s.selectionId : next.id,
        dirty: s.savedKey !== serialize(next),
      });
    },

    applySaved(version) {
      const s = store.getState();
      store.setState({
        version,
        savedKey: serialize(s.tree),
        dirty: false,
      });
    },
  };
}

/** Экспорт для тестов/отладки: выдача уникального id. */
export function newId(): string {
  return uid();
}