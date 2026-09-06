import { describe, expect, it } from 'vitest';
import { fireEvent, screen, waitFor } from '@testing-library/react';
import { jsonResponse, renderApp } from '../test-utils';

interface Dep {
  name: string;
  spec: string;
  resolvedVersion?: string;
}

const LOCK = {
  deps: [
    {
      name: 'a',
      spec: '^1',
      version: '1.0.0',
      integrity: 'sha512-a',
      hoisted: true,
      requestedBy: ['site'],
      peerDependencies: { r: '^1' },
    },
    {
      name: 'x',
      spec: '^2',
      version: '2.0.0',
      integrity: 'sha512-x',
      hoisted: true,
      requestedBy: ['site'],
    },
    {
      name: 'b',
      spec: '^1',
      version: '1.2.0',
      integrity: 'sha512-b',
      hoisted: true,
      requestedBy: ['a@1.0.0'],
    },
    {
      name: 'b',
      spec: '^2',
      version: '2.0.0',
      integrity: 'sha512-b2',
      requestedBy: ['x@2.0.0'],
    },
  ],
};

describe('SiteDepsPage', () => {
  it('грузит и показывает объявленные зависимости и лимит кэша', async () => {
    renderApp({
      path: '/sites/s1/deps',
      handler: (url) => {
        if (url === '/api/sites/s1/dependencies') return jsonResponse(200, [{ name: 'lodash', spec: '^4' }]);
        if (url === '/api/sites/s1/cache-config') return jsonResponse(200, { maxDepsBytes: 0 });
        return jsonResponse(200, {});
      },
    });

    await screen.findByText(/lodash/);
    expect(screen.getByText(/\^4/)).toBeTruthy();
    expect(await screen.findByText('Свой лимит не задан (0)')).toBeTruthy();
  });

  it('добавляет зависимость через форму и список перезагружается', async () => {
    const deps: Dep[] = [];
    const { calls } = await renderApp({
      path: '/sites/s1/deps',
      handler: (url, init) => {
        if (url === '/api/sites/s1/dependencies' && init.method === 'GET') return jsonResponse(200, deps);
        if (url === '/api/sites/s1/dependencies' && init.method === 'POST') {
          const body = JSON.parse(String(init.body)) as Dep;
          deps.push({ name: body.name, spec: body.spec, resolvedVersion: '4.17.21' });
          return jsonResponse(201, { name: body.name, spec: body.spec, resolvedVersion: '4.17.21' });
        }
        if (url === '/api/sites/s1/cache-config') return jsonResponse(200, { maxDepsBytes: 0 });
        return jsonResponse(200, {});
      },
    });

    await screen.findByText(/Зависимостей пока нет/);
    fireEvent.change(screen.getByLabelText('Пакет'), { target: { value: 'lodash' } });
    fireEvent.change(screen.getByLabelText('Диапазон версии'), { target: { value: '^4' } });
    fireEvent.click(screen.getByRole('button', { name: 'Добавить зависимость' }));

    await waitFor(() => {
      expect(calls.some((c) => c.url === '/api/sites/s1/dependencies' && c.method === 'POST')).toBe(true);
    });
    await screen.findByText(/lodash/);
    expect(screen.getByText(/4\.17\.21/)).toBeTruthy();
  });

  it('resolve строит дерево: hoisted/nested/peer бейджи, раскрытие вложенности', async () => {
    renderApp({
      path: '/sites/s1/deps',
      handler: (url, init) => {
        if (url === '/api/sites/s1/dependencies' && init.method === 'GET')
          return jsonResponse(200, [
            { name: 'a', spec: '^1' },
            { name: 'x', spec: '^2' },
          ]);
        if (url === '/api/sites/s1/dependencies/resolve' && init.method === 'POST')
          return jsonResponse(200, LOCK);
        if (url === '/api/sites/s1/cache-config') return jsonResponse(200, { maxDepsBytes: 0 });
        return jsonResponse(200, {});
      },
    });

    await screen.findByText('a'); // список объявленных зависимостей
    fireEvent.click(screen.getByRole('button', { name: 'Разрешить граф' }));

    // дерево: корни a и x (hoisted), a объявляет peer-зависимость
    await screen.findAllByText('hoisted');
    expect(screen.getAllByText('hoisted')).toHaveLength(2); // a, x
    expect(screen.getAllByText('peer')).toHaveLength(1); // a
    // b-инстансы вложены (b@1.2.0 под a, b@2.0.0 под x) и скрыты
    expect(screen.queryByText('b')).toBeNull();

    // раскрываем a → b@1.2.0 (hoisted)
    fireEvent.click(screen.getAllByRole('button', { name: '▸' })[0]!);
    await screen.findByText('b');
    expect(screen.getAllByText('hoisted')).toHaveLength(3); // a, x, b@1.2.0

    // раскрываем x → b@2.0.0 (nested)
    fireEvent.click(screen.getAllByRole('button', { name: '▸' })[0]!);
    await screen.findByText('nested');
    expect(screen.getAllByText('b')).toHaveLength(2);
    expect(screen.getAllByText('nested')).toHaveLength(1);
  });

  it('удаляет зависимость (DELETE /dependencies/{name})', async () => {
    let deps: Dep[] = [{ name: 'lodash', spec: '^4' }];
    const { calls } = await renderApp({
      path: '/sites/s1/deps',
      handler: (url, init) => {
        if (url === '/api/sites/s1/dependencies' && init.method === 'GET') return jsonResponse(200, deps);
        if (url === '/api/sites/s1/dependencies/lodash' && init.method === 'DELETE') {
          deps = [];
          return jsonResponse(204);
        }
        if (url === '/api/sites/s1/cache-config') return jsonResponse(200, { maxDepsBytes: 0 });
        return jsonResponse(200, {});
      },
    });

    await screen.findByText(/lodash/);
    fireEvent.click(screen.getByRole('button', { name: 'Удалить' }));

    await waitFor(() => {
      expect(calls.some((c) => c.url === '/api/sites/s1/dependencies/lodash' && c.method === 'DELETE')).toBe(true);
    });
    await screen.findByText(/Зависимостей пока нет/);
  });

  it('сохраняет лимит кэша (PUT /cache-config) и очищает кэш (POST /evict)', async () => {
    const { calls } = await renderApp({
      path: '/sites/s1/deps',
      handler: (url, init) => {
        if (url === '/api/sites/s1/dependencies') return jsonResponse(200, []);
        if (url === '/api/sites/s1/cache-config' && init.method === 'GET') return jsonResponse(200, { maxDepsBytes: 0 });
        if (url === '/api/sites/s1/cache-config' && init.method === 'PUT')
          return jsonResponse(200, { maxDepsBytes: JSON.parse(String(init.body)).maxDepsBytes });
        if (url === '/api/sites/s1/cache-config/evict' && init.method === 'POST')
          return jsonResponse(200, { evicted: 2, evictedBytes: 2048 });
        return jsonResponse(200, {});
      },
    });

    await screen.findByText('Свой лимит не задан (0)');
    fireEvent.change(screen.getByLabelText('Лимит, байт'), { target: { value: '1048576' } });
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }));

    await waitFor(() => {
      expect(
        calls.some(
          (c) => c.url === '/api/sites/s1/cache-config' && c.method === 'PUT' && String(c.init.body).includes('1048576'),
        ),
      ).toBe(true);
    });

    fireEvent.click(screen.getByRole('button', { name: 'Очистить кэш' }));
    await screen.findByText(/Удалено 2 тарболов/);
  });
});