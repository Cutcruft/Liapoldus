import { describe, expect, it } from 'vitest';
import type { ElementNode } from '../../runtime';
import { createEditorStore, editorActions } from './page-store';
import { findElement, resetUid } from './tree-utils';

const LIST: ElementNode[] = [
  { id: 't1', componentId: 'Text', props: { text: { kind: 'literal', value: 'Привет' } } },
];

const load = () => {
  const store = createEditorStore();
  return { store, actions: editorActions(store) };
};

describe('page-store', () => {
  it('начальное состояние пустое, не грязное', () => {
    const { store } = load();
    expect(store.getState().status).toBe('empty');
    expect(store.getState().dirty).toBe(false);
  });

  it('applyLoaded фиксирует лист и версию, выбирает первый элемент', () => {
    const { store, actions } = load();
    actions.applyLoaded({ list: LIST, version: 3 });
    const s = store.getState();
    expect(s.status).toBe('ready');
    expect(s.version).toBe(3);
    expect(s.selectionId).toBe('t1');
    expect(s.dirty).toBe(false);
  });

  it('insert добавляет элемент в конец и выбирает его', () => {
    resetUid();
    const { store, actions } = load();
    actions.applyLoaded({ list: LIST, version: 1 });
    actions.insert('Button', { type: 'object', properties: {} });
    const s = store.getState();
    expect(s.elements.length).toBe(2);
    expect(s.elements[1]?.id).toBe('n1');
    expect(s.elements[1]?.componentId).toBe('Button');
    expect(s.selectionId).toBe('n1');
    expect(s.dirty).toBe(true);
  });

  it('undo/redo возвращают лист и грязь, applySaved сбрасывает', () => {
    const { store, actions } = load();
    actions.applyLoaded({ list: LIST, version: 1 });
    actions.insert('Button', { type: 'object', properties: {} });

    actions.undo();
    let s = store.getState();
    expect(s.elements.length).toBe(1);
    expect(s.future.length).toBe(1);
    expect(s.dirty).toBe(false);

    actions.redo();
    s = store.getState();
    expect(s.elements.length).toBe(2);
    expect(s.past.length).toBe(1);
    expect(s.dirty).toBe(true);

    actions.applySaved(2);
    s = store.getState();
    expect(s.version).toBe(2);
    expect(s.dirty).toBe(false);
    expect(s.savedKey).toBe(JSON.stringify({ list: s.elements, layoutSectionId: s.layoutSectionId, head: s.head }));
  });

  it('updateProp правит literal-проп элемента и помечает dirty', () => {
    const { store, actions } = load();
    actions.applyLoaded({ list: LIST, version: 1 });
    actions.updateProp('t1', 'text', 'Мир');
    expect(findElement(store.getState().elements, 't1')?.props['text']).toEqual({
      kind: 'literal',
      value: 'Мир',
    });
    expect(store.getState().dirty).toBe(true);
  });

it('setBinding сохраняет binding-источник в props как binding', () => {
    const { store, actions } = load();
    actions.applyLoaded({ list: LIST, version: 1 });
    actions.setBinding('t1', 'text', { kind: 'content', contentId: 'strings', field: 'hero' });
    expect(findElement(store.getState().elements, 't1')?.props['text']).toEqual({
      kind: 'binding',
      source: { kind: 'content', contentId: 'strings', field: 'hero' },
    });
    expect(store.getState().dirty).toBe(true);
  });

  it('applyLoaded переносит layoutSectionId и head, чистая страница', () => {
    const { store, actions } = load();
    const head = { title: 'Заг', og: { image: 'https://x/i.png' } };
    actions.applyLoaded({ list: LIST, version: 4, layoutSectionId: 'Section', head });
    const s = store.getState();
    expect(s.layoutSectionId).toBe('Section');
    expect(s.head).toEqual(head);
    expect(s.dirty).toBe(false);
  });

  it('setLayout меняет каркас и помечает dirty; обратно — снова чисто', () => {
    const { store, actions } = load();
    actions.applyLoaded({ list: LIST, version: 1, layoutSectionId: 'Section' });
    actions.setLayout('Section2');
    let s = store.getState();
    expect(s.layoutSectionId).toBe('Section2');
    expect(s.dirty).toBe(true);
    actions.setLayout('Section');
    s = store.getState();
    expect(s.layoutSectionId).toBe('Section');
    expect(s.dirty).toBe(false);
  });

  it('setHead мержит по ключу, пустые скаляры чистит, помечает dirty', () => {
    const { store, actions } = load();
    actions.applyLoaded({ list: LIST, version: 1 });
    actions.setHead({ title: 'Новый' });
    let s = store.getState();
    expect(s.head.title).toBe('Новый');
    expect(s.dirty).toBe(true);

    actions.setHead({ description: 'Описание' });
    s = store.getState();
    expect(s.head).toEqual({ title: 'Новый', description: 'Описание' });

    actions.setHead({ title: '' });
    s = store.getState();
    expect(s.head).toEqual({ description: 'Описание' });
  });

  it('setHead заменяет og/meta целиком, не мержит ключи', () => {
    const { store, actions } = load();
    actions.applyLoaded({ list: LIST, version: 1, head: { og: { image: 'https://x/a.png' } } });
    actions.setHead({ og: { image: 'https://x/b.png', title: 'T' } });
    expect(store.getState().head.og).toEqual({ image: 'https://x/b.png', title: 'T' });
  });

  it('remove удаляет элемент, выбор переходит на соседний', () => {
    const { store, actions } = load();
    actions.applyLoaded({ list: LIST, version: 1 });
    actions.select('t1');
    actions.remove('t1');
    const s = store.getState();
    expect(s.elements.length).toBe(0);
    expect(s.selectionId).toBeUndefined();
    expect(s.dirty).toBe(true);
  });

  it('remove неизвестного id — no-op', () => {
    const { store, actions } = load();
    actions.applyLoaded({ list: LIST, version: 1 });
    actions.remove('nope');
    expect(store.getState().elements.length).toBe(1);
    expect(store.getState().dirty).toBe(false);
  });

  it('move переставляет местами соседние элементы', () => {
    resetUid();
    const { store, actions } = load();
    actions.applyLoaded({ list: LIST, version: 1 });
    actions.insert('Button', { type: 'object', properties: {} });
    actions.move('n1', 'up');
    const ids = store.getState().elements.map((e) => e.id);
    expect(ids).toEqual(['n1', 't1']);
  });

  it('setLoadError переводит статус в error', () => {
    const { store, actions } = load();
    actions.setLoadError('boom');
    const s = store.getState();
    expect(s.status).toBe('error');
    expect(s.error).toBe('boom');
  });
});