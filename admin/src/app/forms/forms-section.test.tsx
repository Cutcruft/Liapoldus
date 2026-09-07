import { afterEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import type { AdminForm, Submission } from '../../runtime';
import { jsonResponse, renderApp } from '../test-utils';
import { resetSiteTabStore } from '../site-tab-store';
import { buildSubmissionsCsv } from './form-utils';

const SITE = { id: 's1', name: 'Alpha', slug: 'alpha', defaultLocale: 'ru', hosts: [] as string[] };

const FORMS: AdminForm[] = [
  {
    id: 'form.contact',
    siteId: 's1',
    name: 'Contact',
    definition: {
      id: 'form.contact',
      fields: [
        { name: 'email', type: 'email', required: true },
        { name: 'city', type: 'text' },
      ],
      submit: { endpoint: 'form.contact' },
    },
  },
  { id: 'form.booking', siteId: 's1', name: 'Booking', definition: { id: 'form.booking', fields: [], submit: { endpoint: 'form.booking' } } },
];

const SUBMISSIONS: Submission[] = [
  { id: 'sub1', siteId: 's1', formId: 'form.contact', payload: { email: 'a@b.c', city: 'Minsk' }, createdAt: '2026-01-01T00:00:00Z' },
  { id: 'sub2', siteId: 's1', formId: 'form.contact', payload: { email: 'x@y.z', city: 'Warsaw' }, createdAt: '2026-02-01T00:00:00Z' },
];

function formsHandler() {
  let forms = [...FORMS];
  let submissions = [...SUBMISSIONS];
  return (url: string, init: RequestInit) => {
    const u = new URL(url, 'http://localhost');
    const path = u.pathname;
    const method = (init.method ?? 'GET').toUpperCase();
    if (method === 'GET' && path === '/api/sites/s1') return jsonResponse(200, SITE);
    if (method === 'GET' && path === '/api/sites/s1/forms') return jsonResponse(200, forms);
    if (method === 'POST' && path === '/api/sites/s1/forms') {
      const body = JSON.parse(String(init.body)) as { name: string; definition: AdminForm['definition'] };
      const created: AdminForm = { id: body.definition.id, siteId: 's1', name: body.name, definition: body.definition };
      forms = [...forms, created];
      return jsonResponse(201, created);
    }
    if (method === 'DELETE' && path.startsWith('/api/sites/s1/forms/form.booking')) {
      forms = forms.filter((f) => f.id !== 'form.booking');
      return jsonResponse(204, undefined);
    }
    if (path === '/api/sites/s1/forms/form.contact') {
      return jsonResponse(200, forms.find((f) => f.id === 'form.contact') ?? 404);
    }
    if (method === 'GET' && path === '/api/sites/s1/forms/form.contact/submissions')
      return jsonResponse(200, submissions);
    if (method === 'DELETE' && path.startsWith('/api/sites/s1/forms/form.contact/submissions/')) {
      const id = path.split('/').pop()!;
      submissions = submissions.filter((s) => s.id !== id);
      return jsonResponse(204, undefined);
    }
    return jsonResponse(404, { error: `no mock: ${method} ${path}` });
  };
}

afterEach(() => {
  resetSiteTabStore();
  vi.restoreAllMocks();
});

describe('Формы — список (R4)', () => {
  it('показывает формы: имя, id, число полей', async () => {
    await renderApp({ path: '/sites/s1?mode=forms', handler: formsHandler() });

    expect(await screen.findByRole('heading', { name: 'Формы' })).toBeTruthy();
    expect(await screen.findByText('Contact')).toBeTruthy();
    expect(screen.getByText('form.contact')).toBeTruthy();
    expect(screen.getByText('Booking')).toBeTruthy();
    expect(screen.getAllByText('2').length).toBeGreaterThan(0);
  });

  it('создаёт форму: POST name + дефолтный definition (form.<slug>)', async () => {
    const { calls } = await renderApp({ path: '/sites/s1?mode=forms', handler: formsHandler() });

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
    const { calls } = await renderApp({ path: '/sites/s1?mode=forms', handler: formsHandler() });

    expect(await screen.findByText('Booking')).toBeTruthy();
    fireEvent.click(screen.getAllByRole('button', { name: 'Удалить форму' })[1]!);
    fireEvent.click(screen.getByRole('button', { name: 'Подтвердить' }));

    await waitFor(() => expect(screen.queryByText('Booking')).toBeNull());
    expect(calls.some((c) => c.method === 'DELETE' && c.url === '/api/sites/s1/forms/form.booking')).toBe(true);
  });
});

describe('Формы — ответы (R4)', () => {
  it('открытие формы ведёт в ?mode=forms&formId=... и показывает сабмиты', async () => {
    const { router } = await renderApp({ path: '/sites/s1?mode=forms', handler: formsHandler() });

    fireEvent.click(await screen.findByRole('button', { name: 'Contact' }));

    expect(router.state.location.search).toBe('?mode=forms&formId=form.contact');
    expect(await screen.findByText('a@b.c')).toBeTruthy();
    expect(screen.getByText('Minsk')).toBeTruthy();
    expect(screen.getByText('x@y.z')).toBeTruthy();
    expect(screen.getByText('2026-02-01T00:00:00Z')).toBeTruthy();
    expect(screen.getAllByText('JSON').length).toBeGreaterThan(0);
  });

  it('кнопка «К списку» возвращает в ?mode=forms (без formId)', async () => {
    const { router } = await renderApp({ path: '/sites/s1?mode=forms&formId=form.contact', handler: formsHandler() });

    fireEvent.click(await screen.findByRole('button', { name: '← К списку форм' }));

    expect(router.state.location.search).toBe('?mode=forms');
    expect(await screen.findByRole('heading', { name: 'Формы' })).toBeTruthy();
  });

  it('удаление сабмиссии шлёт DELETE и убирает строку', async () => {
    const { calls } = await renderApp({ path: '/sites/s1?mode=forms&formId=form.contact', handler: formsHandler() });

    await screen.findByText('a@b.c');
    fireEvent.click(screen.getAllByRole('button', { name: 'Удалить ответ' })[0]!);
    fireEvent.click(screen.getByRole('button', { name: 'Подтвердить' }));

    await waitFor(() => expect(screen.queryByText('a@b.c')).toBeNull());
    expect(calls.some((c) => c.method === 'DELETE' && c.url === '/api/sites/s1/forms/form.contact/submissions/sub1')).toBe(true);
  });

  it('экспорт CSV генерирует Blob с UTF-8 BOM и скачивает', async () => {
    const createURL = vi.fn<(blob: Blob) => string>(() => 'blob:mock');
    const revokeURL = vi.fn<(url: string) => void>();
    URL.createObjectURL = createURL as typeof URL.createObjectURL;
    URL.revokeObjectURL = revokeURL as typeof URL.revokeObjectURL;
    const clickSpy = vi.fn();
    const originalCreateElement = document.createElement.bind(document);
    vi.spyOn(document, 'createElement').mockImplementation((tag: string) => {
      const el = originalCreateElement(tag);
      if (tag === 'a') (el as HTMLAnchorElement).click = clickSpy;
      return el;
    });

    await renderApp({ path: '/sites/s1?mode=forms&formId=form.contact', handler: formsHandler() });
    await screen.findByText('a@b.c');

    fireEvent.click(screen.getByRole('button', { name: 'Экспорт CSV' }));

    expect(createURL).toHaveBeenCalled();
    const blob = createURL.mock.calls[0]![0]! as Blob;
    const text = await new Promise<string>((resolve, reject) => {
      const reader = new FileReader();
      reader.onload = () => resolve(String(reader.result));
      reader.onerror = () => reject(new Error('FileReader error'));
      reader.readAsText(blob);
    });
    const csv = buildSubmissionsCsv(
      ['email', 'city'],
      [
        { id: 'sub1', createdAt: '2026-01-01T00:00:00Z', payload: { email: 'a@b.c', city: 'Minsk' } },
        { id: 'sub2', createdAt: '2026-02-01T00:00:00Z', payload: { email: 'x@y.z', city: 'Warsaw' } },
      ],
      { date: 'Дата', id: 'ID' },
    );
    expect(text).toBe(csv);
    expect(blob.size).toBe(new TextEncoder().encode(`\uFEFF${csv}`).length);
    expect(clickSpy).toHaveBeenCalled();
  });
});