import { useEffect } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { boot, type BootOptions, type BootRuntime } from '../core/boot';
import { componentMapFromRegistry } from './builtin';
import { RuntimeProvider, useRuntime } from './context';
import { PageRenderer, type ComponentMap } from './render';

export interface MountOptions extends BootOptions {
  /** корень монтирования: элемент, id-селектор или null → `#root`. */
  root?: HTMLElement | string;
  /** карта компонентов (default — ComponentRegistry: builtin + site-определения). */
  components?: ComponentMap;
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

/**
 * Монтирует собранную страницу (Этап 5): boot() + RuntimeProvider +
 * PageRenderer поверх дерева из контракта. Компоненты берутся из
 * ComponentRegistry (builtin + определения site-бандла) — entry.tsx
 * регистрирует их до вызова mount.
 *
 * `baseUrl` по умолчанию — origin текущей страницы (контракт и /build живут на
 * одном сервере); в node/тестах передаётся явно.
 */
export function mount(siteId: string, environment = 'production', opts: MountOptions = {}): Promise<MountResult> {
  const { root, components, ...bootOptions } = opts;
  const baseUrl = bootOptions.baseUrl ?? (typeof location !== 'undefined' ? location.origin : undefined);
  const runtimePromise = boot(siteId, environment, { ...bootOptions, baseUrl });
  const container = rootElement(root);
  const el = createRoot(container);
  const map = components ?? componentMapFromRegistry();

  el.render(
    <RuntimeProvider runtime={runtimePromise}>
      <BindingRefresh />
      <PageRenderer components={map} />
    </RuntimeProvider>,
  );

  return runtimePromise.then((runtime) => ({ runtime, root: el }));
}