import { describe, expect, it } from 'vitest';
import { fireEvent, screen } from '@testing-library/react';
import { jsonResponse, renderApp } from '../test-utils';

const FORM = {
  id: 'form.contact',
  siteId: 's1',
  name: 'Contact',
  definition: {
    id: 'form.contact',
    fields: [{ name: 'email', type: 'email', required: true }],
    submit: { endpoint: 'form.contact' },
  },
};

const SUBMISSION = {
  id: 'subm_1',
  formId: 'form.contact',
  siteId: 's1',
  payload: { email: 'a@b.c' },
  createdAt: '2026-09-05T12:00:00Z',
};

function handler(url: string, init: RequestInit) {
  if (init.method === 'PUT' && url === '/api/sites/s1/forms/form.contact') {
    return jsonResponse(200, { ...FORM, ...(JSON.parse(String(init.body)) as Record<string, unknown>) });
  }
  if (init.method === 'DELETE' && url === '/api/sites/s1/forms/form.contact') {
    return jsonResponse(204, undefined);
  }
  if (url === '/api/sites/s1/forms/form.contact/submissions') return jsonResponse(200, [SUBMISSION]);
  if (url === '/api/sites/s1/forms/form.contact') return jsonResponse(200, FORM);
  return jsonResponse(200, []);
}

describe('FormEditorPage', () => {
  it('загружает форму: имя и определение в JSON', async () => {
    await renderApp({ path: '/sites/s1/forms/form.contact', handler });

    expect(await screen.findByText('К списку форм')).toBeTruthy();
    const textarea = screen.getByLabelText('Определение (JSON)') as HTMLTextAreaElement;
    expect(textarea.value).toContain('"id": "form.contact"');
    expect(textarea.value).toContain('"type": "email"');
    const nameInput = screen.getByLabelText('Название *') as HTMLInputElement;
    expect(nameInput.value).toBe('Contact');
  });

  it('битый JSON → ошибка валидации, PUT не уходит', async () => {
    const { calls } = await renderApp({ path: '/sites/s1/forms/form.contact', handler });

    const textarea = (await screen.findByLabelText('Определение (JSON)')) as HTMLTextAreaElement;
    fireEvent.change(textarea, { target: { value: '{broken' } });
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить форму' }));

    expect(await screen.findByText(/Ошибка определения/)).toBeTruthy();
    expect(calls.some((c) => c.method === 'PUT')).toBe(false);
  });

  it('валидное определение: PUT {name, definition}', async () => {
    const { calls } = await renderApp({ path: '/sites/s1/forms/form.contact', handler });

    const nameInput = (await screen.findByLabelText('Название *')) as HTMLInputElement;
    fireEvent.change(nameInput, { target: { value: 'Contact v2' } });
    const textarea = screen.getByLabelText('Определение (JSON)') as HTMLTextAreaElement;
    fireEvent.change(textarea, {
      target: { value: JSON.stringify({ ...FORM.definition, fields: [{ name: 'msg', type: 'textarea' }] }) },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить форму' }));

    expect(await screen.findByText('Сохранено')).toBeTruthy();

    const put = calls.find((c) => c.method === 'PUT');
    expect(JSON.parse(String(put?.init.body))).toEqual({
      name: 'Contact v2',
      definition: { id: 'form.contact', fields: [{ name: 'msg', type: 'textarea' }], submit: { endpoint: 'form.contact' } },
    });
  });

  it('удаление формы → возврат к списку', async () => {
    await renderApp({ path: '/sites/s1/forms/form.contact', handler });

    fireEvent.click(await screen.findByRole('button', { name: 'Удалить форму' }));
    fireEvent.click(screen.getByRole('button', { name: 'Подтвердить' }));

    expect(await screen.findByText('Форм пока нет')).toBeTruthy();
  });

  it('таб «Сабмиты»: список сабмитов с payload', async () => {
    await renderApp({ path: '/sites/s1/forms/form.contact', handler });

    expect(await screen.findByText('К списку форм')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Сабмиты' }));

    expect(await screen.findByText('subm_1')).toBeTruthy();
    expect(screen.getAllByText(/a@b\.c/).length).toBeGreaterThan(0);
    expect(screen.getByText('2026-09-05T12:00:00Z')).toBeTruthy();
  });
});