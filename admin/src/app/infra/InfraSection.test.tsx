import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, screen, waitFor } from '@testing-library/react';
import { renderApp, jsonResponse, type MockCall } from '../test-utils';
import { makeTranslate, STRINGS } from '../../runtime';

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

const t = makeTranslate(STRINGS);

const SITE = { id: 's1', name: 'Demo', slug: 'demo', defaultLocale: 'ru', hosts: [] };

const OPERATIONS = [
  {
    id: 'content.get',
    siteId: 's1',
    system: true,
    provider: '',
    typeOp: 'query',
    method: 'GET',
    path: '/api/contents/{contentId}',
    cache: 'immutable' as const,
    scope: 'public',
    resultType: 'content',
    params: { in: 'path', fields: { contentId: { required: true } } },
  },
  {
    id: 'contact.save',
    siteId: 's1',
    system: false,
    provider: 'cms',
    typeOp: 'mutation',
    method: 'POST',
    path: '/api/contacts',
    cache: 'disabled' as const,
    scope: 'server',
    resultType: '',
    params: { in: 'query', fields: { name: { required: true } } },
  },
];

const ENDPOINTS = [
  { id: 'booking', siteId: 's1', system: false, method: 'POST', path: '/booking', operationId: 'contact.save' },
];

const FORMS = [
  { id: 'form.contact', siteId: 's1', name: 'Contact', definition: { id: 'form.contact', fields: [], submit: { endpoint: 'form.contact' } } },
];

async function renderInfra() {
  const handler = (url: string, init: RequestInit, calls: MockCall[]): Response => {
    if (url === '/api/sites/s1' && (init.method ?? 'GET') === 'GET') return jsonResponse(200, SITE);
    if (url === '/api/sites/s1/operations' && (init.method ?? 'GET') === 'GET') return jsonResponse(200, OPERATIONS);
    if (url === '/api/sites/s1/endpoints' && (init.method ?? 'GET') === 'GET') return jsonResponse(200, ENDPOINTS);
    if (url === '/api/sites/s1/forms' && (init.method ?? 'GET') === 'GET') return jsonResponse(200, FORMS);
    if (url === '/api/sites/s1/forms/form.contact' && (init.method ?? 'GET') === 'PUT') {
      return jsonResponse(200, { id: 'form.contact', name: 'Contact', definition: FORMS[0]!.definition });
    }
    for (const op of OPERATIONS) {
      if (url === op.path && (init.method ?? 'GET') === op.method) {
        return jsonResponse(op.method === 'POST' ? 201 : 200, { ok: true, echo: url });
      }
    }
    return jsonResponse(404, { error: 'not found: ' + url + ' ' + (init.method ?? 'GET') });
  };
  return renderApp({ path: '/sites/s1?view=editor&section=infra', handler });
}

describe('InfraSection: операции', () => {
  it('показывает таблицу операций и системные бейджи', async () => {
    renderInfra();
    expect(await screen.findByText('content.get')).toBeTruthy();
    expect(screen.getByText(t('infra.systemBadge'))).toBeTruthy();
    expect(screen.getByText(t('infra.editableBadge'))).toBeTruthy();
    expect(screen.getByText('/api/contents/{contentId}')).toBeTruthy();
    expect(screen.getByText('/api/contacts')).toBeTruthy();
  });

  it('создание операции: форма → POST /operations', async () => {
    const { calls } = await renderInfra();
    await screen.findByText('content.get');
    fireEvent.click(screen.getByRole('button', { name: t('infra.newOperation') }));

    fireEvent.change(screen.getByLabelText(/^ID/), { target: { value: 'reviews.list' } });
    fireEvent.change(screen.getByLabelText(/^Путь */), { target: { value: '/api/reviews' } });
    fireEvent.click(screen.getByRole('button', { name: t('infra.create') }));

    await waitFor(() => {
      const create = calls.find((c) => c.url === '/api/sites/s1/operations' && c.method === 'POST');
      expect(create).toBeTruthy();
      const body = JSON.parse(create!.init.body as string);
      expect(body).toMatchObject({ id: 'reviews.list', path: '/api/reviews', typeOp: 'query', method: 'GET', cache: 'disabled' });
    });
  });

  it('предпросмотр query-операции: подстановка параметра и GET по пути runtime', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
    const { calls } = await renderInfra();
    await screen.findByText('content.get');
    fireEvent.click(screen.getAllByRole('button', { name: t('infra.preview') })[0]!);

    fireEvent.change(screen.getByLabelText(/^contentId/), { target: { value: 'hero' } });
    fireEvent.click(screen.getByRole('button', { name: t('infra.preview.run') }));

    await waitFor(() => {
      const call = calls.find((c) => c.url === '/api/contents/hero' && c.method === 'GET');
      expect(call).toBeTruthy();
    });
    expect(confirmSpy).not.toHaveBeenCalled();
    expect(await screen.findByText(/hero/)).toBeTruthy();
  }, 10000);

  it('предпросмотр мутации: сначала confirm, потом POST', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
    const { calls } = await renderInfra();
    await screen.findByText('contact.save');
    const previewButtons = screen.getAllByRole('button', { name: t('infra.preview') });
    fireEvent.click(previewButtons[1]!);

    fireEvent.change(screen.getByLabelText(/^name/), { target: { value: 'Anna' } });
    fireEvent.click(screen.getByRole('button', { name: t('infra.preview.run') }));

    await waitFor(() => expect(confirmSpy).toHaveBeenCalled());
    await waitFor(() => {
      const call = calls.find((c) => c.url === '/api/contacts?name=Anna' && c.method === 'POST');
      expect(call).toBeTruthy();
    });
  }, 10000);

  it('системная операция: нет кнопки удаления', async () => {
    renderInfra();
    await screen.findByText('content.get');
    expect(screen.getAllByRole('button', { name: t('infra.delete') })).toHaveLength(1);
  });
});

describe('InfraSection: эндпоинты', () => {
  it('переключает таб и показывает эндпоинты', async () => {
    renderInfra();
    await screen.findByText('content.get');
    fireEvent.click(screen.getByRole('tab', { name: t('infra.endpoints') }));
    expect(await screen.findByText('booking')).toBeTruthy();
    expect(screen.getByText('/booking')).toBeTruthy();
  });

  it('создание эндпоинта: POST /endpoints c operationId', async () => {
    const { calls } = await renderInfra();
    await screen.findByText('content.get');
    fireEvent.click(screen.getByRole('tab', { name: t('infra.endpoints') }));
    await screen.findByText('booking');

    fireEvent.click(screen.getByRole('button', { name: t('infra.newEndpoint') }));
    fireEvent.change(screen.getByLabelText(/^ID/), { target: { value: 'book2' } });
    fireEvent.change(screen.getByLabelText(/^Путь */), { target: { value: '/book2' } });
    fireEvent.change(screen.getByLabelText(/^Операция */), { target: { value: 'contact.save' } });
    fireEvent.click(screen.getByRole('button', { name: t('infra.create') }));

    await waitFor(() => {
      const create = calls.find((c) => c.url === '/api/sites/s1/endpoints' && c.method === 'POST');
      expect(create).toBeTruthy();
      expect(JSON.parse(create!.init.body as string)).toEqual({
        id: 'book2',
        method: 'POST',
        path: '/book2',
        operationId: 'contact.save',
      });
    });
  });
});

describe('InfraSection: формы (определение)', () => {
  it('submit.target строится из операций и эндпоинтов; сохранение шлёт updateForm', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
    const alertSpy = vi.spyOn(window, 'alert').mockImplementation(() => undefined);
    const { calls } = await renderInfra();
    await screen.findByText('content.get');
    fireEvent.click(screen.getByRole('tab', { name: t('infra.forms') }));
    await screen.findByText('form.contact');

    fireEvent.click(screen.getByRole('button', { name: 'form.contact' }));
    const targetSelect = screen.getByLabelText(/^submit.target/);
    const options = Array.from(targetSelect.querySelectorAll('option')).map((o) => o.getAttribute('value'));
    expect(options).toContain('operation.content.get');
    expect(options).toContain('operation.contact.save');
    expect(options).toContain('endpoint.booking');

    fireEvent.change(targetSelect, { target: { value: 'operation.contact.save' } });
    fireEvent.click(screen.getByRole('button', { name: t('infra.save') }));

    await waitFor(() => {
      const update = calls.find((c) => c.url === '/api/sites/s1/forms/form.contact' && c.method === 'PUT');
      expect(update).toBeTruthy();
      const body = JSON.parse(update!.init.body as string);
      expect(body.definition.submit).toEqual({ target: 'operation.contact.save' });
      expect(body.name).toBe('Contact');
    });
    expect(alertSpy).toHaveBeenCalled();
  });
});