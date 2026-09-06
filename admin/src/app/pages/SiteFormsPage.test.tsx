import { describe, expect, it } from 'vitest';
import { fireEvent, screen, waitFor } from '@testing-library/react';
import type { AdminForm } from '../../runtime';
import { jsonResponse, renderApp } from '../test-utils';

const FORMS: AdminForm[] = [
  {
    id: 'form.contact',
    siteId: 's1',
    name: 'Contact',
    definition: { id: 'form.contact', fields: [{ name: 'email', type: 'email', required: true }], submit: { endpoint: 'form.contact' } },
  },
  { id: 'form.booking', siteId: 's1', name: 'Booking', definition: { id: 'form.booking', fields: [], submit: { endpoint: 'form.booking' } } },
];

describe('SiteFormsPage', () => {
  it('показывает формы: имя, id, число полей', async () => {
    renderApp({
      path: '/sites/s1/forms',
      handler: (url) => (url === '/api/sites/s1/forms' ? jsonResponse(200, FORMS) : jsonResponse(200, [])),
    });

    expect(await screen.findByText('Contact')).toBeTruthy();
    expect(screen.getByText('form.contact')).toBeTruthy();
    expect(screen.getByText('Booking')).toBeTruthy();
    expect(screen.getAllByText('1').length).toBeGreaterThan(0);
    expect(screen.getAllByText('0').length).toBeGreaterThan(0);
  });

  it('создаёт форму: POST name + дефолтный definition (form.<slug>)', async () => {
    let list: AdminForm[] = [...FORMS];
    const { calls } = await renderApp({
      path: '/sites/s1/forms',
      handler: (url, init) => {
        if (url === '/api/sites/s1/forms' && init.method === 'POST') {
          const body = JSON.parse(String(init.body)) as { name: string; definition: AdminForm['definition'] & { fields: AdminForm['definition']['fields'] } };
          const created: AdminForm = { id: body.definition.id, siteId: 's1', name: body.name, definition: body.definition };
          list = [...list, created];
          return jsonResponse(201, created);
        }
        return jsonResponse(200, list);
      },
    });

    fireEvent.click(await screen.findByRole('button', { name: 'Создать форму' }));
    fireEvent.change(screen.getByLabelText('Название *'), { target: { value: 'Feedback Form' } });
    fireEvent.click(screen.getByRole('button', { name: 'Создать' }));

    expect(await screen.findByText('Feedback Form')).toBeTruthy();
    expect(screen.getByText('form.feedback.form')).toBeTruthy();

    const post = calls.find((c) => c.method === 'POST' && c.url === '/api/sites/s1/forms');
    expect(JSON.parse(String(post?.init.body))).toEqual({
      name: 'Feedback Form',
      definition: { id: 'form.feedback.form', fields: [], submit: { endpoint: 'form.feedback.form' } },
    });
  });

  it('удаляет форму через подтверждение', async () => {
    let list: AdminForm[] = [...FORMS];
    const { calls } = await renderApp({
      path: '/sites/s1/forms',
      handler: (url, init) => {
        if (init.method === 'DELETE' && url === '/api/sites/s1/forms/form.booking') {
          list = list.filter((f) => f.id !== 'form.booking');
          return jsonResponse(204, undefined);
        }
        return jsonResponse(200, list);
      },
    });

    expect(await screen.findByText('Booking')).toBeTruthy();
    fireEvent.click(screen.getAllByRole('button', { name: 'Удалить форму' })[1]!);
    fireEvent.click(screen.getByRole('button', { name: 'Подтвердить' }));

    await waitFor(() => expect(screen.queryByText('Booking')).toBeNull());
    expect(calls.some((c) => c.method === 'DELETE' && c.url === '/api/sites/s1/forms/form.booking')).toBe(true);
  });

  it('ссылка «Открыть» ведёт в редактор формы', async () => {
    renderApp({
      path: '/sites/s1/forms',
      handler: (url) => {
        if (url === '/api/sites/s1/forms/form.contact') return jsonResponse(200, FORMS[0]);
        return jsonResponse(200, FORMS);
      },
    });

    fireEvent.click((await screen.findAllByRole('link', { name: 'Открыть' }))[0]!);
    expect((await screen.findAllByText('Определение (JSON)')).length).toBeGreaterThan(0);
  });
});