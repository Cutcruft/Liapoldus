import { describe, expect, it } from 'vitest';
import { fireEvent, screen, waitFor } from '@testing-library/react';
import { jsonResponse, renderApp } from '../test-utils';

describe('SiteAllowlistPage', () => {
  it('грузит и показывает правила allowlist', async () => {
    renderApp({
      path: '/sites/s1/allowlist',
      handler: (url) => {
        if (url === '/api/sites/s1/dependencies/allowlist') return jsonResponse(200, ['lodash', '@acme/*']);
        return jsonResponse(200, []);
      },
    });

    await screen.findByText('lodash');
    expect(screen.getByText('@acme/*')).toBeTruthy();
  });

  it('добавляет правило (POST allowlist) и список перезагружается', async () => {
    const entries: string[] = [];
    const { calls } = await renderApp({
      path: '/sites/s1/allowlist',
      handler: (url, init) => {
        if (url === '/api/sites/s1/dependencies/allowlist' && init.method === 'GET') return jsonResponse(200, entries);
        if (url === '/api/sites/s1/dependencies/allowlist' && init.method === 'POST') {
          const body = JSON.parse(String(init.body)) as { entry: string };
          entries.push(body.entry);
          return jsonResponse(201, { entry: body.entry });
        }
        return jsonResponse(200, []);
      },
    });

    await screen.findByText('Правил пока нет — разрешены все пакеты');
    fireEvent.change(screen.getByLabelText('Правило'), { target: { value: '@scope/*' } });
    fireEvent.click(screen.getByRole('button', { name: 'Добавить правило' }));

    await waitFor(() => {
      expect(
        calls.some(
          (c) => c.url === '/api/sites/s1/dependencies/allowlist' && c.method === 'POST',
        ),
      ).toBe(true);
    });
    await screen.findByText('@scope/*');
  });

  it('удаляет правило (DELETE allowlist?entry=)', async () => {
    let entries: string[] = ['lodash'];
    const { calls } = await renderApp({
      path: '/sites/s1/allowlist',
      handler: (url, init) => {
        if (url === '/api/sites/s1/dependencies/allowlist' && init.method === 'GET') return jsonResponse(200, entries);
        if (url.includes('/dependencies/allowlist') && init.method === 'DELETE') {
          entries = [];
          return jsonResponse(204);
        }
        return jsonResponse(200, []);
      },
    });

    await screen.findByText('lodash');
    fireEvent.click(screen.getByRole('button', { name: 'Удалить' }));

    await waitFor(() => {
      const del = calls.find((c) => c.url.includes('/dependencies/allowlist') && c.method === 'DELETE');
      expect(del?.url).toContain('entry=lodash');
    });
    await screen.findByText('Правил пока нет — разрешены все пакеты');
  });
});