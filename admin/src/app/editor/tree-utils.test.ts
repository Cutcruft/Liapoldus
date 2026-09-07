import { describe, expect, it } from 'vitest';
import type { ElementNode } from '../../runtime';
import {
  elementSummary,
  findElement,
  indexOfElement,
  insertElement,
  makeElement,
  moveElement,
  removeElement,
  replaceElement,
  resetUid,
  uid,
} from './tree-utils';
import { BUILTIN_BY_TYPE } from './schemas';

const LIST: ElementNode[] = [
  { id: 'e1', componentId: 'Text', props: { text: { kind: 'literal', value: 'A' } } },
  { id: 'e2', componentId: 'Button', props: { label: { kind: 'literal', value: 'Go' } } },
];

describe('list-utils (tree-utils: линейная модель)', () => {
  it('findElement/indexOfElement находят элемент по id', () => {
    expect(findElement(LIST, 'e2')?.componentId).toBe('Button');
    expect(indexOfElement(LIST, 'e1')).toBe(0);
    expect(findElement(LIST, 'missing')).toBeUndefined();
    expect(indexOfElement(LIST, 'missing')).toBe(-1);
  });

  it('replaceElement заменяет элемент, сохраняя остальные и позицию', () => {
    const next = replaceElement(LIST, 'e1', { id: 'e1', componentId: 'Text', props: { text: { kind: 'literal', value: 'B' } } });
    expect(next[0]?.props['text']).toEqual({ kind: 'literal', value: 'B' });
    expect(next[1]?.id).toBe('e2');
    expect(replaceElement(LIST, 'nope', { id: 'x', componentId: 'Text', props: {} })).toBe(LIST);
  });

  it('insertElement добавляет в конец и по индексу', () => {
    const el: ElementNode = { id: 'n1', componentId: 'Text', props: {} };
    const appended = insertElement(LIST, el);
    expect(appended.length).toBe(3);
    expect(appended[2]?.id).toBe('n1');

    const at0 = insertElement(LIST, { ...el, id: 'n0' }, 0);
    expect(at0[0]?.id).toBe('n0');
    expect(at0[1]?.id).toBe('e1');
  });

  it('insertElement по индексу за границами клампится', () => {
    const el: ElementNode = { id: 'x', componentId: 'Text', props: {} };
    expect(insertElement(LIST, el, 99)[2]?.id).toBe('x');
    expect(insertElement(LIST, el, -5)[0]?.id).toBe('x');
  });

  it('removeElement удаляет элемент, не трогая остальные', () => {
    const removed = removeElement(LIST, 'e1');
    expect(removed.length).toBe(1);
    expect(removed[0]?.id).toBe('e2');
    expect(removeElement(LIST, 'nope')).toBe(LIST);
  });

  it('moveElement переставляет соседей вверх/вниз, не выходит за границы', () => {
    const up = moveElement(LIST, 'e2', 'up');
    expect(up.map((e) => e.id)).toEqual(['e2', 'e1']);
    expect(moveElement(LIST, 'e1', 'up')).toBe(LIST);
    expect(moveElement(LIST, 'e2', 'down')).toBe(LIST);
    expect(moveElement(LIST, 'nope', 'up')).toBe(LIST);
  });

  it('uid уникален и сбрасывается', () => {
    resetUid();
    expect(uid()).toBe('n1');
    expect(uid()).toBe('n2');
    resetUid();
    expect(uid()).toBe('n1');
  });

  it('makeElement сливает дефолты схемы как литералы', () => {
    const el = makeElement('Text', BUILTIN_BY_TYPE['Text']?.schema);
    expect(el.componentId).toBe('Text');
    expect(el.props['text']).toEqual({ kind: 'literal', value: 'Текст' });
    expect(el.props['size']).toEqual({ kind: 'literal', value: 'md' });
  });

  it('elementSummary берёт label/text/title литералы', () => {
    expect(elementSummary(LIST[0]!)).toBe('A');
    expect(elementSummary({ id: 'x', componentId: 'Text', props: {} })).toBeUndefined();
    expect(
      elementSummary({
        id: 'y',
        componentId: 'Text',
        props: { label: { kind: 'literal', value: 'Меню' }, text: { kind: 'literal', value: 'Игнор' } },
      }),
    ).toBe('Меню');
  });
});