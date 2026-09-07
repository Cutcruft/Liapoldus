import { createSliceStore, type SliceStore } from '@liapoldus/ui-runtime';
import type { BindingSource, ElementNode, ElementProp } from '../../runtime';
import {
  findElement,
  insertElement,
  makeElement,
  moveElement,
  removeElement,
  replaceElement,
  uid,
} from './tree-utils';
import type { JSONSchema } from './schemas';

export interface EditorState {
  status: 'empty' | 'ready' | 'error';
  /** Упорядоченный лист элементов (позиция = порядок рендера). */
  elements: ElementNode[];
  selectionId?: string;
  past: ElementNode[][];
  future: ElementNode[][];
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
    elements: [],
    past: [],
    future: [],
    version: 0,
    savedKey: null,
    dirty: false,
  });
}

const serialize = (list: ElementNode[]): string => JSON.stringify(list);
const HISTORY_CAP = 50;

export interface EditorActions {
  applyLoaded(page: { list: ElementNode[]; version: number }): void;
  setLoadError(message: string): void;
  select(id: string): void;
  /** Добавляет элемент указанного типа (в конец листа). */
  insert(componentId: string, schema?: JSONSchema): void;
  remove(id: string): void;
  move(id: string, dir: 'up' | 'down'): void;
  updateProp(id: string, field: string, value: unknown): void;
  setBinding(id: string, field: string, source: BindingSource): void;
  undo(): void;
  redo(): void;
  applySaved(version: number): void;
}

export function editorActions(store: EditorStore): EditorActions {
  const commit = (nextElements: ElementNode[]) => {
    const s = store.getState();
    if (serialize(s.elements) === serialize(nextElements)) return;
    const past = [...s.past, s.elements].slice(-HISTORY_CAP);
    store.setState({
      elements: nextElements,
      past,
      future: [],
      dirty: s.savedKey !== serialize(nextElements),
      // selection остаётся, если элемент ещё существует, иначе — первый элемент
      selectionId:
        s.selectionId && findElement(nextElements, s.selectionId)
          ? s.selectionId
          : nextElements[0]?.id,
    });
  };

  return {
    applyLoaded(page) {
      const s = store.getState();
      store.setState({
        status: 'ready',
        elements: page.list,
        version: page.version,
        savedKey: serialize(page.list),
        dirty: false,
        past: [],
        future: [],
        selectionId: page.list[0]?.id,
        error: undefined,
      });
      void s;
    },

    setLoadError(message) {
      store.setState({ status: 'error', error: message });
    },

    select(id) {
      const s = store.getState();
      if (findElement(s.elements, id)) store.setState({ selectionId: id });
    },

    insert(componentId, schema) {
      const s = store.getState();
      const element = makeElement(componentId, schema);
      commit(insertElement(s.elements, element));
      store.setState({ selectionId: element.id });
    },

    remove(id) {
      const s = store.getState();
      commit(removeElement(s.elements, id));
    },

    move(id, dir) {
      const s = store.getState();
      commit(moveElement(s.elements, id, dir));
    },

    updateProp(id, field, value) {
      const s = store.getState();
      const el = findElement(s.elements, id);
      if (!el) return;
      const props: Record<string, ElementProp> = { ...el.props, [field]: { kind: 'literal', value } };
      const next: ElementNode = { ...el, props };
      commit(replaceElement(s.elements, id, next));
    },

    setBinding(id, field, source) {
      const s = store.getState();
      const el = findElement(s.elements, id);
      if (!el) return;
      const props: Record<string, ElementProp> = { ...el.props, [field]: { kind: 'binding', source } };
      const next: ElementNode = { ...el, props };
      commit(replaceElement(s.elements, id, next));
    },

    undo() {
      const s = store.getState();
      if (s.past.length === 0) return;
      const prev = s.past[s.past.length - 1]!;
      store.setState({
        elements: prev,
        past: s.past.slice(0, -1),
        future: [s.elements, ...s.future].slice(0, HISTORY_CAP),
        selectionId:
          s.selectionId && findElement(prev, s.selectionId)
            ? s.selectionId
            : prev[0]?.id,
        dirty: s.savedKey !== serialize(prev),
      });
    },

    redo() {
      const s = store.getState();
      if (s.future.length === 0) return;
      const next = s.future[0]!;
      store.setState({
        elements: next,
        past: [...s.past, s.elements].slice(-HISTORY_CAP),
        future: s.future.slice(1),
        selectionId:
          s.selectionId && findElement(next, s.selectionId)
            ? s.selectionId
            : next[0]?.id,
        dirty: s.savedKey !== serialize(next),
      });
    },

    applySaved(version) {
      const s = store.getState();
      store.setState({
        version,
        savedKey: serialize(s.elements),
        dirty: false,
      });
    },
  };
}

/** Экспорт для тестов/отладки: выдача уникального id. */
export function newId(): string {
  return uid();
}
