import { afterEach, describe, expect, it } from 'vitest';
import { fireEvent, screen, within } from '@testing-library/react';
import { renderApp, jsonResponse } from './test-utils';
import { resetTabs } from './tabs-store';
import { resetSiteTabStore } from './site-tab-store';

const SITE = { id: 's1', name: 'Alpha', slug: 'alpha', defaultLocale: 'ru', hosts: [] as string[] };

function siteHandler() {
  return jsonResponse(200, SITE);
}

afterEach(() => {
  resetTabs();
  resetSiteTabStore();
});

function modeCluster() {
  return screen.getByRole('tablist', { name: 'Режим' });
}

describe('SiteTab — режимы вкладки сайта (R2)', () => {
  it('по умолчанию открывается «Обслуживание» с панелью подразделов', async () => {
    const { router } = await renderApp({ path: '/sites/s1', handler: siteHandler });

    expect(await screen.findByRole('heading', { name: 'Alpha' })).toBeTruthy();

    const cluster = modeCluster();
    expect(within(cluster).getByRole('tab', { name: 'Обслуживание' }).getAttribute('aria-selected')).toBe('true');
    expect(within(cluster).getByRole('tab', { name: 'Редактор' }).getAttribute('aria-selected')).toBe('false');

    const maintNav = screen.getByRole('navigation', { name: 'Обслуживание' });
    expect(within(maintNav).getByRole('button', { name: 'Контент' }).getAttribute('aria-pressed')).toBe('true');
    expect(within(maintNav).getByRole('button', { name: 'Формы+ответы' })).toBeTruthy();
    expect(within(maintNav).getByRole('button', { name: 'Медиа' })).toBeTruthy();

    expect(await screen.findByRole('heading', { name: 'Контент' })).toBeTruthy();
    expect(router.state.location.search).toBe('');
  });

  it('тумблер переключает в «Редактор», URL получает view=editor и section', async () => {
    const { router } = await renderApp({ path: '/sites/s1', handler: siteHandler });
    await screen.findByRole('heading', { name: 'Alpha' });

    fireEvent.click(within(modeCluster()).getByRole('tab', { name: 'Редактор' }));

    expect(await screen.findByRole('heading', { name: 'Компоненты' })).toBeTruthy();
    const editorNav = screen.getByRole('navigation', { name: 'Редактор' });
    expect(within(editorNav).getByRole('button', { name: 'Компоненты' }).getAttribute('aria-pressed')).toBe('true');
    expect(router.state.location.search).toBe('?view=editor&section=components');
  });

  it('выбор раздела редактирует section в URL и показывает его заглушку', async () => {
    const { router } = await renderApp({ path: '/sites/s1?view=editor', handler: siteHandler });
    await screen.findByRole('heading', { name: 'Alpha' });

    fireEvent.click(within(screen.getByRole('navigation', { name: 'Редактор' })).getByRole('button', { name: 'Страницы' }));

    expect(await screen.findByRole('heading', { name: 'Страницы' })).toBeTruthy();
    expect(router.state.location.search).toBe('?view=editor&section=pages');
  });

  it('возврат из «Обслуживания» в «Редактор» открывает запомненный раздел', async () => {
    const { router } = await renderApp({ path: '/sites/s1', handler: siteHandler });
    await screen.findByRole('heading', { name: 'Alpha' });
    const cluster = modeCluster();

    fireEvent.click(within(cluster).getByRole('tab', { name: 'Редактор' }));
    await screen.findByRole('heading', { name: 'Компоненты' });
    fireEvent.click(within(screen.getByRole('navigation', { name: 'Редактор' })).getByRole('button', { name: 'Страницы' }));
    await screen.findByRole('heading', { name: 'Страницы' });

    fireEvent.click(within(cluster).getByRole('tab', { name: 'Обслуживание' }));
    await screen.findByRole('heading', { name: 'Контент' });
    expect(router.state.location.search).toBe('?mode=content');

    fireEvent.click(within(cluster).getByRole('tab', { name: 'Редактор' }));
    expect(await screen.findByRole('heading', { name: 'Страницы' })).toBeTruthy();
    expect(router.state.location.search).toBe('?mode=content&view=editor&section=pages');
  });

  it('глубокая ссылка восстанавливает режим и раздел из URL', async () => {
    const { router } = await renderApp({ path: '/sites/s1?view=editor&section=pages', handler: siteHandler });

    expect(await screen.findByRole('heading', { name: 'Alpha' })).toBeTruthy();
    expect(within(modeCluster()).getByRole('tab', { name: 'Редактор' }).getAttribute('aria-selected')).toBe('true');
    expect(await screen.findByRole('heading', { name: 'Страницы' })).toBeTruthy();
    expect(router.state.location.search).toBe('?view=editor&section=pages');
  });

  it('в редакторе без section используется запомненный раздел (по умолчанию «Компоненты»)', async () => {
    await renderApp({ path: '/sites/s1?view=editor', handler: siteHandler });
    expect(await screen.findByRole('heading', { name: 'Компоненты' })).toBeTruthy();
  });

  it('ошибка сайта показывает сообщение и кнопку на главную, без режимной панели', async () => {
    const { router } = await renderApp({
      path: '/sites/missing',
      handler: () => jsonResponse(404, { error: 'site not found' }),
    });

    expect(await screen.findByText(/site not found/)).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Сайты' }));
    expect(router.state.location.pathname).toBe('/');
  });
});

describe('SiteTab — подразделы обслуживания в URL (R3)', () => {
  it('выбор подраздела «Формы+ответы» меняет mode в URL и показывает список форм', async () => {
    const { router } = await renderApp({ path: '/sites/s1', handler: siteHandler });
    await screen.findByRole('heading', { name: 'Alpha' });

    fireEvent.click(within(screen.getByRole('navigation', { name: 'Обслуживание' })).getByRole('button', { name: 'Формы+ответы' }));

    expect(await screen.findByRole('heading', { name: 'Формы' })).toBeTruthy();
    expect(router.state.location.search).toBe('?mode=forms');
  });

  it('глубокая ссылка ?mode=media открывает Медиа и подсвечивает подраздел', async () => {
    const { router } = await renderApp({ path: '/sites/s1?mode=media', handler: siteHandler });
    await screen.findByRole('heading', { name: 'Alpha' });

    expect(await screen.findByRole('heading', { name: 'Медиа' })).toBeTruthy();
    const maintNav = screen.getByRole('navigation', { name: 'Обслуживание' });
    expect(within(maintNav).getByRole('button', { name: 'Медиа' }).getAttribute('aria-pressed')).toBe('true');
    expect(router.state.location.search).toBe('?mode=media');
  });

  it('возврат в «Обслуживание» сохраняет активный подраздел (mode) из памяти/URL', async () => {
    const { router } = await renderApp({ path: '/sites/s1?mode=forms', handler: siteHandler });
    await screen.findByRole('heading', { name: 'Формы' });

    fireEvent.click(within(modeCluster()).getByRole('tab', { name: 'Редактор' }));
    await screen.findByRole('heading', { name: 'Компоненты' });

    fireEvent.click(within(modeCluster()).getByRole('tab', { name: 'Обслуживание' }));
    expect(await screen.findByRole('heading', { name: 'Формы' })).toBeTruthy();
    expect(router.state.location.search).toBe('?mode=forms');
  });

  it('переход в «Редактор» убирает contentId из URL, но сохраняет mode', async () => {
    const { router } = await renderApp({ path: '/sites/s1?mode=content&contentId=c2', handler: siteHandler });
    await screen.findByRole('heading', { name: 'Alpha' });

    fireEvent.click(within(modeCluster()).getByRole('tab', { name: 'Редактор' }));
    await screen.findByRole('heading', { name: 'Компоненты' });
    expect(router.state.location.search).toBe('?mode=content&view=editor&section=components');
  });
});