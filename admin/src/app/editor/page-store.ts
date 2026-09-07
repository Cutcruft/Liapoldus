import { createSliceStore, type SliceStore } from '@liapoldus/ui-runtime';
import type { BindingSource, ElementNode, ElementProp, PageHead } from '../../runtime';
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
  /** Override каркаса (section с acceptsPageContent); '' = default сайта. */
  layoutSectionId: string;
  /** Per-page head override (R10). */
  head: PageHead;
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
    layoutSectionId: '',
    head: {},
    past: [],
    future: [],
    version: 0,
    savedKey: null,
    dirty: false,
  });
}

const HISTORY_CAP = 50;

/** Всё, что переживает сохранение (кроме version): по нему считается dirty. */
export function savePayloadOf(state: {
  list: ElementNode[];
  layoutSectionId: string;
  head: PageHead;
}): { list: ElementNode[]; layoutSectionId: string; head: PageHead } {
  return state;
}

const serialize = (state: { elements: ElementNode[]; layoutSectionId: string; head: PageHead }): string =>
  JSON.stringify({ list: state.elements, layoutSectionId: state.layoutSectionId, head: state.head });

const payloadFor = (list: ElementNode[], layoutSectionId: string, head: PageHead): string =>
  JSON.stringify({ list, layoutSectionId, head });

export interface EditorActions {
  applyLoaded(page: { list: ElementNode[]; version: number; layoutSectionId?: string; head?: PageHead }): void;
  setLoadError(message: string): void;
  select(id: string): void;
  /** Добавляет элемент указанного типа (в конец листа). */
  insert(componentId: string, schema?: JSONSchema): void;
  remove(id: string): void;
  move(id: string, dir: 'up' | 'down'): void;
  updateProp(id: string, field: string, value: unknown): void;
  setBinding(id: string, field: string, source: BindingSource): void;
  /** Устанавливает каркас страницы ('' — default сайта). */
  setLayout(sectionId: string): void;
  /** Обновляет per-page head (мерж по ключу; пустые скаляры удаляются). */
  setHead(patch: Partial<PageHead>): void;
  undo(): void;
  redo(): void;
  applySaved(version: number): void;
}

/** Пустые скаляры/пустые og/meta убираем — они не перекрывают site-дефолты. */
function cleanHead(head: PageHead): PageHead {
  const out: PageHead = {};
  if (head.title) out.title = head.title;
  if (head.description) out.description = head.description;
  if (head.robots) out.robots = head.robots;
  if (head.canonical) out.canonical = head.canonical;
  if (head.og && Object.keys(head.og).length > 0) out.og = head.og;
  if (head.meta && Object.keys(head.meta).length > 0) out.meta = head.meta;
  return out;
}

export function editorActions(store: EditorStore): EditorActions {
  const commit = (nextElements: ElementNode[]) => {
    const s = store.getState();
    if (serialize(s) === payloadFor(nextElements, s.layoutSectionId, s.head)) return;
    const past = [...s.past, s.elements].slice(-HISTORY_CAP);
    store.setState({
      elements: nextElements,
      past,
      future: [],
      dirty: s.savedKey !== payloadFor(nextElements, s.layoutSectionId, s.head),
      // selection остаётся, если элемент ещё существует, иначе — первый элемент
      selectionId:
        s.selectionId && findElement(nextElements, s.selectionId)
          ? s.selectionId
          : nextElements[0]?.id,
    });
  };

  const commitHead = (patch: Partial<PageHead>) => {
    const s = store.getState();
    const merged: PageHead = {
      ...s.head,
      ...patch,
      og: patch.og ?? s.head.og,
      meta: patch.meta ?? s.head.meta,
    };
    const head = cleanHead(merged);
    if (serialize(s) === payloadFor(s.elements, s.layoutSectionId, head)) return;
    const past = [...s.past, s.elements].slice(-HISTORY_CAP);
    store.setState({
      head,
      past,
      future: [],
      version: s.version,
      dirty: s.savedKey !== payloadFor(s.elements, s.layoutSectionId, head),
    });
  };

  return {
    applyLoaded(page) {
      store.setState({
        status: 'ready',
        elements: page.list,
        layoutSectionId: page.layoutSectionId ?? '',
        head: cleanHead(page.head ?? {}),
        version: page.version,
        savedKey: JSON.stringify(
          savePayloadOf({ list: page.list, layoutSectionId: page.layoutSectionId ?? '', head: cleanHead(page.head ?? {}) }),
        ),
        dirty: false,
        past: [],
        future: [],
        selectionId: page.list[0]?.id,
        error: undefined,
      });
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

    setLayout(sectionId) {
      const s = store.getState();
      const nextLayout = sectionId ?? '';
      if (nextLayout === s.layoutSectionId) return;
      const past = [...s.past, s.elements].slice(-HISTORY_CAP);
      store.setState({
        layoutSectionId: nextLayout,
        past,
        future: [],
        dirty: s.savedKey !== payloadFor(s.elements, nextLayout, s.head),
      });
    },

    setHead(patch) {
      commitHead(patch);
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
        dirty: s.savedKey !== payloadFor(prev, s.layoutSectionId, s.head),
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
        dirty: s.savedKey !== payloadFor(next, s.layoutSectionId, s.head),
      });
    },

    applySaved(version) {
      const s = store.getState();
      store.setState({
        version,
        savedKey: serialize(s),
        dirty: false,
      });
    },
  };
}

/** Экспорт для тестов/отладки: выдача уникального id. */
export function newId(): string {
  return uid();
}
