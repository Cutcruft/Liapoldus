import { useEffect, useState } from 'react';
import { useSelector } from '@liapoldus/ui-runtime';
import type { DevRebuildEvent, SliceStore, Translate } from '../../runtime';
import { connectDevWs } from '../ws-client';
import type { PreviewActions, PreviewState } from './preview-store';
import { ToolButton } from './controls';

const SINGLE_PANE_BTN = 'rounded-t-md border border-b-0 px-3 py-1 text-xs';

/**
 * Живое canvas-превью собранной страницы: iframe на `/build/{site}/{env}/{snapshot}/dist/index.html`,
 * авто-рефреш по `DevRebuildEvent` из `/dev/build/ws`.
 */
export function CanvasPreview({
  siteId,
  previewStore,
  preview,
  t,
}: {
  siteId: string;
  previewStore: SliceStore<PreviewState>;
  preview: PreviewActions;
  t: Translate;
}) {
  const status = useSelector(previewStore, (s) => s.status);
  const url = useSelector(previewStore, (s) => s.url);
  const error = useSelector(previewStore, (s) => s.error);
  const building = useSelector(previewStore, (s) => s.inFlight);

  const [frameKey, setFrameKey] = useState(0);
  const [activeUrl, setActiveUrl] = useState<string | undefined>(url);

  useEffect(() => {
    if (url) setActiveUrl(url);
  }, [url]);

  useEffect(() => {
    const off = connectDevWs(siteId, (event: DevRebuildEvent) => {
      preview.applyEvent(event);
      if (event.status === 'ready') setFrameKey((k) => k + 1);
    });
    return off;
  }, [siteId, preview]);

  return (
    <div className="flex h-full flex-col gap-2">
      <div className="flex items-center gap-2 text-xs">
        <span
          className={
            status === 'failed'
              ? 'text-red-600'
              : status === 'building' || (building && status === 'idle')
                ? 'text-neutral-400'
                : 'text-emerald-600'
          }
        >
          {status === 'failed'
            ? t('preview.failed', { error: error ?? '' })
            : status === 'building' || building
              ? t('preview.building')
              : t('preview.ready')}
        </span>
        <div className="ml-auto flex items-center gap-1">
          {url && (
            <a
              href={url}
              target="_blank"
              rel="noreferrer"
              className="rounded border border-neutral-300 px-2 py-1 hover:bg-neutral-50"
            >
              {t('preview.open')}
            </a>
          )}
          <ToolButton title={t('preview.build')} onClick={() => void preview.requestBuild()} disabled={building}>
            {t('preview.build')}
          </ToolButton>
          {url && (
            <ToolButton title={t('preview.refresh')} onClick={() => setFrameKey((k) => k + 1)}>
              ↻
            </ToolButton>
          )}
        </div>
      </div>

      <div className="flex-1 overflow-auto rounded border border-neutral-200 bg-neutral-50">
        {activeUrl ? (
          <iframe title={t('preview.ready')} key={frameKey} src={activeUrl} className="h-full w-full" />
        ) : (
          <div className="flex h-full items-center justify-center p-6 text-center text-sm text-neutral-400">
            {t('preview.empty')}
          </div>
        )}
      </div>
    </div>
  );
}

/** Переключатель «Дизайн | Превью» над центром-панелью. */
export function CanvasTabBar({
  view,
  onView,
  t,
}: {
  view: 'design' | 'preview';
  onView: (view: 'design' | 'preview') => void;
  t: Translate;
}) {
  return (
    <div className="mb-2 flex gap-1 text-sm">
      {(
        [
          ['design', t('canvas.design')],
          ['preview', t('canvas.preview')],
        ] as const
      ).map(([key, label]) => (
        <button
          key={key}
          type="button"
          onClick={() => onView(key)}
          className={
            view === key
              ? `${SINGLE_PANE_BTN} border-blue-500 bg-blue-50 text-blue-700`
              : `${SINGLE_PANE_BTN} border-transparent text-neutral-500 hover:text-neutral-800`
          }
        >
          {label}
        </button>
      ))}
    </div>
  );
}