import { useCallback, useSyncExternalStore } from 'react';
import type { SliceStore } from '../core/slice-store';

/**
 * Подписка React на SliceStore (zustand-обёртка ui-runtime).
 *
 * Контракт: селектор должен возвращать стабильные ссылки по `===` при том же
 * состоянии (store делает immutable-copy в setState) — тогда срабатывают
 * перерисовки только при фактическом изменении выбранного среза.
 */
export function useSelector<T, U>(store: SliceStore<T>, selector: (state: T) => U): U {
  const subscribe = useCallback(
    (cb: () => void) => store.subscribe(cb as (state: T, prev: T) => void),
    [store],
  );
  return useSyncExternalStore(subscribe, () => selector(store.getState()));
}