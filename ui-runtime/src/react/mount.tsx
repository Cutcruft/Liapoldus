import { useEffect, useState, type ReactNode } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { boot, type BootOptions, type BootRuntime } from '../core/boot';
import { componentMapFromRegistry } from './builtin';
import { PageLoader, getPageTree, hasPageTree, resolveBuildBase } from '../core/pages';
import { RuntimeProvider, useRuntime } from './context';
import { PageRenderer, pageIdOf, type ComponentMap } from './render';
import { HeadController } from './head-controller';
import { useRoute } from './hooks';

export interface MountOptions extends BootOptions {
  /** корень монтирования: элемент, id-селектор или null → `#root`. */
  root?: HTMLElement | string;
  /** карта компонентов (default — ComponentRegistry: builtin + зарегистрированные site-определения). */
  components?: ComponentMap;
  /** корень раздачи сборки (manifest.json + чанки); default — вывод из URL/`baseUrl`. */
  buildBaseUrl?: string;
  /** место пока держится placeholder, пока догружается чанк страницы (0-контент ОК). */
  fallback?: ReactNode;
  /**
   * Догрузчик чанков (тесты). default — нативный `import()`.
   * Чанк сам вызывает `registerPage`/ComponentRegistry — загрузчик только ждёт его.
   */
  importer?: (chunkUrl: string) => Promise<unknown>;
}

export interface MountResult {
  runtime: BootRuntime;
  root: Root;
}

function rootElement(ref: MountOptions['root']): HTMLElement {
  if (typeof ref === 'string') {
    const el = document.getElementById(ref);
    if (!el) throw new Error(`mount: элемент '#${ref}' не найден`);
    return el;
  }
  if (ref instanceof HTMLElement) return ref;
  const el = document.getElementById('root');
  if (!el) throw new Error(`mount: элемент '#root' не найден`);
  return el;
}

/**
 * Подписывается на срезы-источники bindings (роут/контент/операции/формы) и на
 * каждое изменение пересчитывает дерево (TreeController.refresh), реализуя
 * «пересборка vs синк данных» на живой сборке (§11, Этап 5).
 */
function BindingRefresh() {
  const runtime = useRuntime();

  useEffect(() => {
    const unsubs = [
      runtime.store.subscribeSlice((s) => s.route, () => runtime.tree.refresh()),
      runtime.store.subscribeSlice((s) => s.content, () => runtime.tree.refresh()),
      runtime.store.subscribeSlice((s) => s.operationResults, () => runtime.tree.refresh()),
      runtime.store.subscribeSlice((s) => s.forms, () => runtime.tree.refresh()),
    ];
    return () => {
      for (const off of unsubs) off();
    };
  }, [runtime]);

  return null;
}

interface CodeSplitPagesProps {
  runtime: BootRuntime;
  loader: PageLoader;
  components?: ComponentMap;
  fallback?: ReactNode;
}

/**
 * Слой код-сплиттинга (§16#8): при переходе на renderPage-роут догружает чанк
 * странице (один раз, кэш), берёт зарегистрированное дерево и переключает
 * store.tree. Пока несколько страниц используют один реестр деревьев — старый
 * контент остаётся на экране (без белых кадров), placeholder показывается только
 * до первого дерева (контракт/dev-WS не отдали его).
 */
function CodeSplitPages({ runtime, loader, components, fallback }: CodeSplitPagesProps) {
  const route = useRoute();
  const pageId = pageIdOf(route);
  const [loadedTick, setLoadedTick] = useState(0);

  useEffect(() => {
    if (!pageId || hasPageTree(pageId)) return;
    let alive = true;
    loader.load(pageId).then(() => {
      if (!alive) return;
      const decl = getPageTree(pageId);
      if (decl && runtime.store.getState().tree?.pageId !== pageId) {
        // R10 P1: чанк несёт raw layout/head per-page; site-дефолт layout
        // подмешивается здесь (у контрактного пути это делает boot.pageDeclaration).
        runtime.tree.load({
          ...decl,
          layoutSectionId: decl.layoutSectionId ?? runtime.defaultLayoutSectionId,
        });
      }
    });
    return () => {
      alive = false;
    };
  }, [pageId, runtime, loader]);

  useEffect(() => loader.onLoaded(() => setLoadedTick((n) => n + 1)), [loader]);

  if (!pageId) return null;
  void loadedTick;
  const map = components ?? componentMapFromRegistry();
  const ready = hasPageTree(pageId) || (runtime.store.getState().tree?.pageId ?? null) === pageId;
  if (!ready && !runtime.store.getState().tree) {
    return fallback ? <>{fallback}</> : null;
  }
  return <PageRenderer components={map} />;
}

/**
 * Монтирует собранный сайт (Этап 5 п.4): boot() + RuntimeProvider + догрузчик
 * код-сплит чанков + PageRenderer.
 *
 * Порядок: boot() берёт контракт (дескрипторы/роуты/тему/стартовое дерево),
 * PageLoader читает manifest.json и гарантирует загрузку домашнего чанка
 * (modulepreload в shell делает его почти мгновенным) — поэтому первый экран
 * рендерится сразу с компонентами сайта, а при навигации чанки остальных
 * страниц догружаются по требованию.
 *
 * `baseUrl` по умолчанию — origin текущей страницы (контракт и /build живут на
 * одном сервере); в node/тестах передаётся явно.
 */
export function mount(siteId: string, environment = 'production', opts: MountOptions = {}): Promise<MountResult> {
  const { root, components, buildBaseUrl, fallback, importer, ...bootOptions } = opts;
  const baseUrl = bootOptions.baseUrl ?? (typeof location !== 'undefined' ? location.origin : undefined);
  const runtimePromise = boot(siteId, environment, { ...bootOptions, baseUrl });
  const container = rootElement(root);
  const el = createRoot(container);
  const buildRoot = resolveBuildBase(baseUrl, siteId, environment, bootOptions.versionId, buildBaseUrl);
  const loader = new PageLoader(buildRoot, bootOptions.env?.fetch, importer);

  el.render(
    <RuntimeProvider runtime={runtimePromise}>
      <ReadyPages loader={loader} components={components} fallback={fallback} />
    </RuntimeProvider>,
  );

  return runtimePromise.then((r) => ({ runtime: r, root: el }));
}

/** Проксирует loader в CodeSplitPages после readiness RuntimeProvider. */
function ReadyPages(props: Omit<CodeSplitPagesProps, 'runtime'>) {
  const runtime = useRuntime();
  return (
    <>
      <BindingRefresh />
      <HeadController />
      <CodeSplitPages runtime={runtime} {...props} />
    </>
  );
}