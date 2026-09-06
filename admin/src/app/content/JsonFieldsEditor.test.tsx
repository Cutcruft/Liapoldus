import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { makeTranslate } from '../../runtime';
import { STRINGS } from '../../runtime/strings';
import { JsonFieldsEditor } from './JsonFieldsEditor';

const t = makeTranslate(STRINGS);
afterEach(cleanup);

describe('JsonFieldsEditor', () => {
  it('рендерит строки из объекта: ключ/тип/значение', () => {
    render(
      <JsonFieldsEditor
        value={{ title: 'Hi', count: 2, on: true, meta: { a: 1 } }}
        onChange={() => {}}
        t={t}
      />,
    );

    const keys = screen.getAllByLabelText('Ключ').map((el) => (el as HTMLInputElement).value);
    expect(keys).toEqual(['title', 'count', 'on', 'meta']);

    const values = screen.getAllByLabelText('Значение');
    expect((values[0] as HTMLInputElement).value).toBe('Hi');
    expect((values[1] as HTMLInputElement).value).toBe('2');
    expect((values[2] as HTMLInputElement).type).toBe('checkbox');
    expect((values[2] as HTMLInputElement).checked).toBe(true);
    expect((values[3] as HTMLInputElement).value).toContain('"a": 1');

    const types = screen.getAllByLabelText('Тип').map((el) => (el as HTMLSelectElement).value);
    expect(types).toEqual(['string', 'number', 'boolean', 'json']);
  });

  it('переименование ключа и правка значения эмитит новый объект', () => {
    const onChange = vi.fn();
    render(<JsonFieldsEditor value={{ title: 'Hi' }} onChange={onChange} t={t} />);

    fireEvent.change(screen.getByLabelText('Ключ'), { target: { value: 'heading' } });
    expect(onChange).toHaveBeenLastCalledWith({ heading: 'Hi' });

    fireEvent.change(screen.getByLabelText('Значение'), { target: { value: 'Yo' } });
    expect(onChange).toHaveBeenLastCalledWith({ heading: 'Yo' });
  });

  it('number: невалидный ввод не ломает объект (держится предыдущее значение)', () => {
    const onChange = vi.fn();
    render(<JsonFieldsEditor value={{ count: 2 }} onChange={onChange} t={t} />);

    const valueInput = screen.getByLabelText('Значение');
    fireEvent.change(valueInput, { target: { value: 'abc' } });
    expect(onChange).toHaveBeenLastCalledWith({ count: 2 });
    expect(screen.getByText('NaN')).toBeTruthy();

    fireEvent.change(valueInput, { target: { value: '42' } });
    expect(onChange).toHaveBeenLastCalledWith({ count: 42 });
  });

  it('json: битый JSON показывает ошибку и держит прежний объект', () => {
    const onChange = vi.fn();
    render(<JsonFieldsEditor value={{ meta: { a: 1 } }} onChange={onChange} t={t} />);

    const valueInput = screen.getByLabelText('Значение');
    fireEvent.change(valueInput, { target: { value: '{broken' } });
    expect(onChange).toHaveBeenLastCalledWith({ meta: { a: 1 } });
    expect(screen.getByText('Поле "meta": невалидный JSON')).toBeTruthy();

    fireEvent.change(valueInput, { target: { value: '{"ok": 1}' } });
    expect(onChange).toHaveBeenLastCalledWith({ meta: { ok: 1 } });
  });

  it('добавление/удаление строки', () => {
    const onChange = vi.fn();
    render(<JsonFieldsEditor value={{}} onChange={onChange} t={t} />);

    fireEvent.click(screen.getByRole('button', { name: /\+\s*Добавить поле/ }));
    expect(onChange).toHaveBeenLastCalledWith({ field1: '' });

    fireEvent.click(screen.getAllByRole('button', { name: '✕' })[0]!);
    expect(onChange).toHaveBeenLastCalledWith({});
  });

  it('смена типа number → выставляет значение по умолчанию', () => {
    const onChange = vi.fn();
    render(<JsonFieldsEditor value={{ x: 'text' }} onChange={onChange} t={t} />);

    fireEvent.change(screen.getByLabelText('Тип'), { target: { value: 'number' } });
    expect(onChange).toHaveBeenLastCalledWith({ x: 0 });
  });
});