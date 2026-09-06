import { describe, expect, it } from 'vitest';
import type { ComponentNode } from '../../runtime';
import {
  findNode,
  findParent,
  insertChild,
  makeNode,
  moveNode,
  nodePath,
  removeNode,
  replaceNode,
  resetUid,
  uid,
} from './tree-utils';
import { BUILTIN_BY_TYPE } from './schemas';

const TREE: ComponentNode = {
  id: 'root',
  type: 'Container',
  props: { layout: 'stack', gap: 8 },
  children: [
    { id: 't1', type: 'Text', props: { text: 'A' } },
    {
      id: 'c2',
      type: 'Container',
      props: { layout: 'row' },
      children: [{ id: 'b1', type: 'Button', props: { label: 'Go' } }],
    },
  ],
};

describe('tree-utils', () => {
  it('findNode находит вложенный узел и не находит отсутствующий', () => {
    expect(findNode(TREE, 'b1')?.type).toBe('Button');
    expect(findNode(TREE, 'missing')).toBeUndefined();
  });

  it('findParent возвращает непосредственного родителя', () => {
    expect(findParent(TREE, 'b1')?.id).toBe('c2');
    expect(findParent(TREE, 'root')).toBeUndefined();
  });

  it('replaceNode заменяет узел, сохраняя остальные', () => {
    const next = replaceNode(TREE, 't1', { id: 't1', type: 'Text', props: { text: 'B' } });
    expect(findNode(next, 't1')?.props?.['text']).toBe('B');
    expect(findNode(next, 'b1')).toBeTruthy();
  });

  it('insertChild добавляет в конец и по индексу', () => {
    const appended = insertChild(TREE, 'root', { id: 'n1', type: 'Text', props: {} });
    expect(appended.children?.length).toBe(3);
    expect(appended.children?.[2]?.id).toBe('n1');

    const at0 = insertChild(TREE, 'root', { id: 'n0', type: 'Text', props: {} }, 0);
    expect(at0.children?.[0]?.id).toBe('n0');
    expect(at0.children?.[1]?.id).toBe('t1');
  });

  it('insertChild в отсутствующий родитель не меняет дерево', () => {
    expect(insertChild(TREE, 'nope', { id: 'x', type: 'Text' })).toBe(TREE);
  });

  it('removeNode удаляет лист и не даёт удалить root', () => {
    const removed = removeNode(TREE, 't1');
    expect(removed.children?.length).toBe(1);
    expect(removed.children?.[0]?.id).toBe('c2');
    expect(removeNode(TREE, 'root')).toBe(TREE);
  });

  it('moveNode переставляет соседей вверх/вниз и не выходит за границы', () => {
    const up = moveNode(TREE, 'c2', 'up');
    expect(up.children?.[0]?.id).toBe('c2');
    expect(up.children?.[1]?.id).toBe('t1');

    expect(moveNode(TREE, 't1', 'up')).toBe(TREE);
    expect(moveNode(TREE, 'c2', 'down')).toBe(TREE);

    const innerDown = moveNode(TREE, 'b1', 'down');
    expect(innerDown).toBe(TREE);
  });

  it('nodePath собирает путь от root до узла', () => {
    const path = nodePath(TREE, 'b1');
    expect(path.map((n) => n.id)).toEqual(['root', 'c2', 'b1']);
    expect(nodePath(TREE, 'nope')).toEqual([]);
  });

  it('uid уникален и сбрасывается', () => {
    resetUid();
    expect(uid()).toBe('n1');
    expect(uid()).toBe('n2');
    resetUid();
    expect(uid()).toBe('n1');
  });

  it('makeNode сливает дефолты схемы и props пусто', () => {
    const node = makeNode('Text', {}, BUILTIN_BY_TYPE['Text']?.schema);
    expect(node.props).toMatchObject({ text: 'Текст', size: 'md' });
    expect(node.bindings).toEqual({});
  });
});