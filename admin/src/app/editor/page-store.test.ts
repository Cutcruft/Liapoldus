import { describe, expect, it } from 'vitest';
import type { ComponentNode } from '../../runtime';
import { createEditorStore, editorActions } from './page-store';
import { findNode, resetUid } from './tree-utils';

const ROOT: ComponentNode = {
  id: 'root',
  type: 'Container',
  props: { layout: 'stack', gap: 8 },
  children: [{ id: 't1', type: 'Text', props: { text: 'Привет' } }],
};

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

  it('applyLoaded фиксирует дерево и версию, выбирает root', () => {
    const { store, actions } = load();
    actions.applyLoaded({ root: ROOT, version: 3 });
    const s = store.getState();
    expect(s.status).toBe('ready');
    expect(s.version).toBe(3);
    expect(s.selectionId).toBe('root');
    expect(s.dirty).toBe(false);
  });

  it('insert добавляет ребёнка настраиваемого типа и выбирает его', () => {
    resetUid();
    const { store, actions } = load();
    actions.applyLoaded({ root: ROOT, version: 1 });
    actions.insert('root', 'Text', { type: 'object', properties: {} });
    const s = store.getState();
    expect(s.tree?.children?.length).toBe(2);
    expect(s.tree?.children?.[1]?.id).toBe('n1');
    expect(s.selectionId).toBe('n1');
    expect(s.dirty).toBe(true);
  });

  it('undo/redo возвращают дерево и грязь, applySaved сбрасывает', () => {
    const { store, actions } = load();
    actions.applyLoaded({ root: ROOT, version: 1 });
    actions.insert('root', 'Text', { type: 'object', properties: {} });

    actions.undo();
    let s = store.getState();
    expect(s.tree?.children?.length).toBe(1);
    expect(s.future.length).toBe(1);
    expect(s.dirty).toBe(false);

    actions.redo();
    s = store.getState();
    expect(s.tree?.children?.length).toBe(2);
    expect(s.past.length).toBe(1);
    expect(s.dirty).toBe(true);

    actions.applySaved(2);
    s = store.getState();
    expect(s.version).toBe(2);
    expect(s.dirty).toBe(false);
    expect(serialize(s.tree)).toBe(s.savedKey);
  });

  it('updateProp правит props узла и помечает dirty', () => {
    const { store, actions } = load();
    actions.applyLoaded({ root: ROOT, version: 1 });
    actions.updateProp('t1', 'text', 'Мир');
    expect(findNode(store.getState().tree!, 't1')?.props?.['text']).toBe('Мир');
    expect(store.getState().dirty).toBe(true);
  });

  it('setBinding сохраняет источник в bindings', () => {
    const { store, actions } = load();
    actions.applyLoaded({ root: ROOT, version: 1 });
    actions.setBinding('t1', 'text', { source: 'content', path: 'strings.hero' });
    expect(findNode(store.getState().tree!, 't1')?.bindings?.['text']).toEqual({
      source: 'content',
      path: 'strings.hero',
    });
    expect(store.getState().dirty).toBe(true);
  });

  it('remove удаляет не-root узел, выбор переходит на root', () => {
    const { store, actions } = load();
    actions.applyLoaded({ root: ROOT, version: 1 });
    actions.select('t1');
    actions.remove('t1');
    const s = store.getState();
    expect(s.tree?.children?.length).toBe(0);
    expect(s.selectionId).toBe('root');
  });

  it('remove(root) — no-op', () => {
    const { store, actions } = load();
    actions.applyLoaded({ root: ROOT, version: 1 });
    actions.remove('root');
    expect(store.getState().tree?.children?.length).toBe(1);
    expect(store.getState().dirty).toBe(false);
  });

  it('move переставляет местами соседние узлы', () => {
    resetUid();
    const { store, actions } = load();
    actions.applyLoaded({ root: ROOT, version: 1 });
    actions.insert('root', 'Text', { type: 'object', properties: {} });
    actions.move('n1', 'up');
    const ids = store.getState().tree?.children?.map((c) => c.id);
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

const serialize = (tree?: ComponentNode) => JSON.stringify(tree);