import { describe, expect, it } from 'vitest';
import { fireEvent, screen, waitFor } from '@testing-library/react';
import { renderApp, jsonResponse } from '../test-utils';

const DASHBOARD = {
  siteCount: 2,
  sites: [
    {
      siteId: 's1',
      name: 'Первый',
      slug: 'first',
      defaultLocale: 'ru',
      hosts: ['a.example','b.example'],
      git: {
        siteId: 's1',
        branches: ['dev', 'main'],
        dev: { existing: true, sha: 'abc1234567de' },
        main: { existing: true, sha: 'def99988877766' },
        dirty: true,
      },
    },
    {
      siteId: 's2',
      name: 'Второй',
      slug: 'second',
      defaultLocale: 'en',
      hosts: [],
      git: {
        siteId: 's2',
        branches: [],
        dev: { existing: false },
        main: { existing: false },
        dirty: false,
      },
    },
  ],
  recentBuilds: [
    { id: 'b1', siteId: 's1', siteName: 'Первый', snapshotId: '', environment: 'prod', status: 'ready', createdAt: '2026-01-01T10:00:00Z' },
    { id: 'b2', siteId: 's2', siteName: 'Второй', snapshotId: '', environment: 'dev', status: 'failed', createdAt: '2026-01-01T09:00:00Z' },
  ],
  recentSnapshots: [
    { id: 'n1', siteId: 's1', siteName: 'Первый', name: 'Старт', gitSha: 'abc1234567de', createdAt: '2026-01-01T08:00:00Z' },
  ],
  runtimeStatus: 'degraded',
};

describe('DashboardPage', () => {
  it('без сайтов — empty-state с CTA', async () => {
    await renderApp({ path: '/' });
    expect(await screen.findByText('Создайте первый сайт')).toBeTruthy();
    expect(screen.getByText('Создать сайт')).toBeTruthy();
  });

  it('агрегирует сайты, git-статус, последние сборки и снапшоты', async () => {
    await renderApp({ path: '/', handler: () => jsonResponse(200, DASHBOARD) });

    expect(await screen.findByRole('heading', { name: 'Обзор' })).toBeTruthy();
    expect(screen.getByText('требует внимания')).toBeTruthy();

    expect(screen.getAllByText('Первый').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText('Второй').length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText(/dev abc12345/)).toBeTruthy();
    expect(screen.getByText(/main def99988/)).toBeTruthy();
    expect(screen.getByText('есть несохранённые')).toBeTruthy();
    expect(screen.getByText('ещё не инициализирован')).toBeTruthy();

    expect(screen.getByText('Последние сборки')).toBeTruthy();
    expect(screen.getByText('ready')).toBeTruthy();
    expect(screen.getByText('failed')).toBeTruthy();
    expect(screen.getAllByText('Последние снапшоты').length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText('Старт')).toBeTruthy();
  });

  it('reload повторяет запрос', async () => {
    const { calls } = await renderApp({ path: '/', handler: () => jsonResponse(200, DASHBOARD) });
    await screen.findByRole('heading', { name: 'Обзор' });
    const before = calls.filter((c) => c.url === '/api/dashboard').length;
    fireEvent.click(screen.getByRole('button', { name: 'Обновить' }));
    await waitFor(() => {
      expect(calls.filter((c) => c.url === '/api/dashboard').length).toBeGreaterThan(before);
    });
  });
});