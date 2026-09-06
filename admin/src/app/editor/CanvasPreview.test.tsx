import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { createAdminApi } from '../../runtime/api';
import { makeTranslate } from '../../runtime/i18n';
import { STRINGS } from '../../runtime/strings';
import { setDevSocketFactory, type WsLike } from '../ws-client';
import { createPreviewStore, previewActions } from './preview-store';
import type { PreviewActions } from './preview-store';
import { CanvasPreview } from './CanvasPreview';

const t = makeTranslate(STRINGS);

function inertSocket(): WsLike {
  return { onmessage: null, onopen: null, onclose: null, onerror: null, close: () => {} };
}

const noopActions: PreviewActions = {
  requestBuild: () => Promise.resolve(),
  applyEvent: () => {},
  clear: () => {},
};

afterEach(() => {
  cleanup();
  setDevSocketFactory(undefined);
  localStorage.clear();
});

beforeEach(() => {
  setDevSocketFactory(() => inertSocket());
});

describe('CanvasPreview', () => {
  it('пустое состояние: подсказка + кнопка сборки вызывает requestBuild', async () => {
    const store = createPreviewStore('s1');
    const requestBuild = vi.fn(() => Promise.resolve());
    render(
      <CanvasPreview
        siteId="s1"
        previewStore={store}
        preview={{ ...noopActions, requestBuild }}
        t={t}
      />,
    );

    fireEvent.click(screen.getByText('Собрать превью'));
    expect(requestBuild).toHaveBeenCalledTimes(1);
  });

  it('персист-снапшот: iframe с путём артефакта + ссылка «в новой вкладке»', () => {
    localStorage.setItem('liapoldus.preview.s1', 'snap9');
    const store = createPreviewStore('s1');
    render(<CanvasPreview siteId="s1" previewStore={store} preview={noopActions} t={t} />);

    const iframe = document.querySelector('iframe') as HTMLIFrameElement | null;
    expect(iframe?.getAttribute('src')).toBe('/build/s1/development/snap9/dist/index.html');
    expect(document.querySelector('a[target="_blank"]')).toBeTruthy();
  });

  it('WS DevRebuildEvent (ready) → applyEvent + пересборка iframe-кадра', async () => {
    localStorage.setItem('liapoldus.preview.s1', 'snap9');
    const store = createPreviewStore('s1');
    const preview = previewActions(store, {
      api: createAdminApi({ baseUrl: '', getToken: () => null, fetchFn: async () => new Response(null, { status: 404 }) }),
      t,
      siteId: 's1',
    });
    const sockets: WsLike[] = [];
    setDevSocketFactory(() => {
      const s = inertSocket();
      sockets.push(s);
      return s;
    });

    render(<CanvasPreview siteId="s1" previewStore={store} preview={preview} t={t} />);

    sockets[0]!.onmessage?.({
      data: JSON.stringify({
        siteId: 's1',
        environment: 'development',
        snapshotId: 'live2',
        status: 'ready',
        updatedAt: 'x',
      }),
    });

    await waitFor(() => {
      const iframe = document.querySelector('iframe') as HTMLIFrameElement | null;
      expect(iframe?.getAttribute('src')).toBe('/build/s1/development/live2/dist/index.html');
    });
  });
});