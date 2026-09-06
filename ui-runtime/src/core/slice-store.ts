import { createStore } from 'zustand/vanilla';

/**
 * Обобщённый slice-store — zustand-обёртка ui-runtime.
 *
 * Админка (и любой клиент ui-runtime) НЕ импортирует zustand напрямую:
 * состояние строится на createSliceStore + useSelector. Частичные обновления
 * мержатся shallow (immutable-copy): каждый setState создаёт новые ссылки на
 * обновлённые срезы, поэтому селекторы в useSelector стабильны по `===`.
 */

/** Частичное обновление: объект-патч или функция от предыдущего состояния. */
export type SliceUpdater<T> = Partial<T> | ((prev: T) => Partial<T>);

export interface SliceStore<T> {
  getState(): T;
  /** shallow-merge патча; функция получает prev и возвращает патч. */
  setState(updater: SliceUpdater<T>): void;
  /** подписка на любые изменения; вернёт отписку. */
  subscribe(listener: (state: T, prev: T) => void): () => void;
  /** slice-подписка: при `===`-неизменной выборке не вызывает listener. */
  subscribeSlice<U>(selector: (s: T) => U, listener: (value: U, prev: U) => void): () => void;
}

export function createSliceStore<T>(initial: T): SliceStore<T> {
  const store = createStore<T>()(() => initial);

  const api: SliceStore<T> = {
    getState: () => store.getState(),

    setState: (updater) => {
      const patch =
        typeof updater === 'function'
          ? (updater as (prev: T) => Partial<T>)(store.getState())
          : updater;
      store.setState({ ...(store.getState() as object), ...(patch as object) } as T);
    },

    subscribe: (listener) => store.subscribe(listener),

    subscribeSlice: (selector, listener) => {
      let prev = selector(store.getState());
      return store.subscribe((state) => {
        const next = selector(state);
        if (next !== prev) {
          listener(next, prev);
          prev = next;
        }
      });
    },
  };

  return api;
}