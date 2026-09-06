import { useCallback, useEffect, useMemo, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { useSelector } from '@liapoldus/ui-runtime';
import { runOperation, type Page } from '../../runtime';
import { useAdmin } from '../admin-context';
import { useOperation } from '../use-operation';
import { createEditorStore, editorActions } from './page-store';
import { TreePanel } from './TreePanel';
import { Inspector } from './Inspector';
import { PreviewPane } from './PreviewPane';
import { CanvasPreview, CanvasTabBar } from './CanvasPreview';
import { createPreviewStore, previewActions } from './preview-store';
import { ToolButton } from './controls';

type SaveState = 'idle' | 'saving' | 'saved' | 'failed';

const AUTOSAVE_MS = 1500;

const PANEL_CLASS = 'rounded-lg border border-neutral-200 bg-white p-3';

export function EditorPage() {
  const { siteId = '', pageId = '' } = useParams();
  const { api, t } = useAdmin();

  const store = useMemo(() => createEditorStore(), []);
  const actions = useMemo(() => editorActions(store), [store]);
  const page = useOperation<Page>('getPage', { pageId });

  const previewStore = useMemo(() => createPreviewStore(siteId), [siteId]);
  const preview = useMemo(() => previewActions(previewStore, { api, t, siteId }), [previewStore, api, t, siteId]);
  const [canvasView, setCanvasView] = useState<'design' | 'preview'>('design');

  const tree = useSelector(store, (s) => s.tree);
  const dirty = useSelector(store, (s) => s.dirty);
  const version = useSelector(store, (s) => s.version);
  const pastCount = useSelector(store, (s) => s.past.length);
  const futureCount = useSelector(store, (s) => s.future.length);
  const loadError = useSelector(store, (s) => s.error);
  const status = useSelector(store, (s) => s.status);

  const [saveState, setSaveState] = useState<SaveState>('idle');

  const performSave = useCallback(async () => {
    const cur = store.getState();
    if (!cur.tree) return;
    setSaveState('saving');
    const res = await runOperation(api, 'saveTree', { pageId, root: cur.tree }, t);
    if (!res.ok) {
      setSaveState('failed');
      return;
    }
    const next = res.data as { version?: number } | undefined;
    actions.applySaved(next?.version ?? cur.version);
    setSaveState('saved');
    void preview.requestBuild();
  }, [api, t, pageId, actions, store, preview]);

  useEffect(() => {
    if (!dirty || !tree) return;
    setSaveState('saving');
    const timer = setTimeout(() => {
      void performSave();
    }, AUTOSAVE_MS);
    return () => {
      clearTimeout(timer);
    };
  }, [dirty, tree, performSave]);

  useEffect(() => {
    if (page.state.status === 'success') {
      actions.applyLoaded({ root: page.state.data.root, version: page.state.data.version });
    } else if (page.state.status === 'error') {
      actions.setLoadError(page.state.detail);
    }
  }, [page.state, actions]);

  if (page.state.status === 'loading') {
    return <p className="p-8 text-sm text-neutral-400">{t('common.loading')}</p>;
  }

  if (page.state.status === 'error' && status !== 'ready' && !tree) {
    return (
      <div className="p-8">
        <p className="text-sm text-red-600">
          {t('common.error')}: {loadError ?? page.state.detail}
        </p>
        <button type="button" onClick={page.reload} className="mt-2 rounded border border-neutral-300 px-3 py-1 text-sm">
          {t('common.reload')}
        </button>
      </div>
    );
  }

  const pageName = page.state.status === 'success' ? page.state.data.name : '';

  return (
    <div className="flex h-full flex-col gap-4 p-4">
      <div className="flex flex-wrap items-center gap-3">
        <Link to={`/sites/${siteId}`} className="text-sm text-neutral-500 hover:text-blue-600">
          ← {pageName || t('nav.pages')}
        </Link>
        <h1 className="text-lg font-medium">{t('editor.title')}</h1>
        <div className="ml-auto flex items-center gap-2">
          <SaveIndicator saveState={saveState} dirty={dirty} version={version} t={t} />
          <ToolButton title={t('editor.saveNow')} onClick={() => void performSave()}>
            {t('editor.saveNow')}
          </ToolButton>
          <ToolButton title={t('editor.undo')} disabled={pastCount === 0} onClick={actions.undo}>
            ↶
          </ToolButton>
          <ToolButton title={t('editor.redo')} disabled={futureCount === 0} onClick={actions.redo}>
            ↷
          </ToolButton>
        </div>
      </div>

      <div className="grid flex-1 grid-cols-[15rem_minmax(0,1fr)_18rem] gap-4 overflow-hidden">
        <section className={`${PANEL_CLASS} overflow-auto`} aria-label={t('editor.tree')}>
          <TreePanel store={store} actions={actions} t={t} />
        </section>
        <section className={`${PANEL_CLASS} overflow-auto`} aria-label={t('editor.canvas')}>
          <CanvasTabBar view={canvasView} onView={setCanvasView} t={t} />
          {canvasView === 'design' ? (
            <PreviewPane store={store} t={t} />
          ) : (
            <CanvasPreview siteId={siteId} previewStore={previewStore} preview={preview} t={t} />
          )}
        </section>
        <section className={`${PANEL_CLASS} overflow-auto`} aria-label={t('editor.props')}>
          <Inspector store={store} actions={actions} t={t} />
        </section>
      </div>
    </div>
  );
}

function SaveIndicator({
  saveState,
  dirty,
  version,
  t,
}: {
  saveState: SaveState;
  dirty: boolean;
  version: number;
  t: ReturnType<typeof useAdmin>['t'];
}) {
  if (saveState === 'failed') return <span className="text-xs text-red-600">{t('editor.saved.failed')}</span>;
  if (saveState === 'saving') return <span className="text-xs text-neutral-400">{t('editor.saving')}</span>;
  if (dirty) return <span className="text-xs text-amber-600">{t('editor.dirty')}</span>;
  return <span className="text-xs text-emerald-600">{t('editor.saved', { version })}</span>;
}