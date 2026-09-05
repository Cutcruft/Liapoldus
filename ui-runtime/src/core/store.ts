import { createStore } from 'zustand/vanilla';
import type { AssetMeta } from '../types/asset';
import type { ContentData } from '../types/content';
import type { RouteDescriptor } from '../types/descriptor';
import type { FormRuntimeSnapshot } from '../types/form';
import type { TreeDeclaration } from '../types/tree';

/** Срезы runtime-состояния (тест-спека §14). */
export interface RuntimeState {
  ready: boolean;
  tree: TreeDeclaration | null;
  routes: RouteDescriptor[];
  tokens: Record<string, string>;
  content: Record<string, ContentData>;
  assets: Record<string, AssetMeta>;
  operationResults: Record<string, unknown>;
  locale: string;
  forms: Record<string, FormRuntimeSnapshot>;
}

export interface RuntimeActions {
  setReady(ready?: boolean): void;
  setTree(tree: TreeDeclaration | null): void;
  setRoutes(routes: RouteDescriptor[]): void;
  applyTokens(tokens: Record<string, string>): void;
  setContent(content: Record<string, ContentData>): void;
  setAssets(assets: Record<string, AssetMeta>): void;
  setOperationResult(key: string, value: unknown): void;
  setLocale(locale: string): void;
  setFormState(formId: string, patch: Partial<FormRuntimeSnapshot>): void;
}

export type RuntimeStoreState = RuntimeState & RuntimeActions;

export interface RuntimeStore {
  getState(): RuntimeStoreState;
  setState(patch: Partial<RuntimeStoreState>): void;
  subscribe(listener: (state: RuntimeStoreState, prevState: RuntimeStoreState) => void): () => void;
  /** Подписка на срез: колбэк вызывается только при изменении выбранного значения (по ===). */
  subscribeSlice<T>(selector: (s: RuntimeState) => T, listener: (value: T, prev: T) => void): () => void;
}

export const defaultStoreState: RuntimeState = {
  ready: false,
  tree: null,
  routes: [],
  tokens: {},
  content: {},
  assets: {},
  operationResults: {},
  locale: '',
  forms: {},
};

const idleForm: FormRuntimeSnapshot = { status: 'idle', values: {}, errors: {} };

/** Создаёт runtime-стор (zustand vanilla). Каждый срез обновляется отдельным действием. */
export function createRuntimeStore(initial?: Partial<RuntimeState>): RuntimeStore {
  const store = createStore<RuntimeStoreState>()((set) => ({
    ...defaultStoreState,
    ...initial,
    setReady: (ready = true) => set({ ready }),
    setTree: (tree) => set({ tree }),
    setRoutes: (routes) => set({ routes }),
    applyTokens: (tokens) => set({ tokens }),
    setContent: (content) => set({ content }),
    setAssets: (assets) => set({ assets }),
    setOperationResult: (key, value) =>
      set((s) => ({ operationResults: { ...s.operationResults, [key]: value } })),
    setLocale: (locale) => set({ locale }),
    setFormState: (formId, patch) =>
      set((s) => ({
        forms: {
          ...s.forms,
          [formId]: { ...idleForm, ...s.forms[formId], ...patch },
        },
      })),
  }));

  const api = store as unknown as RuntimeStore;

  api.subscribeSlice = <T>(selector: (s: RuntimeState) => T, listener: (value: T, prev: T) => void): (() => void) => {
    let prev = selector(store.getState() as RuntimeState);
    return store.subscribe((state, prevState) => {
      const next = selector(state as RuntimeState);
      if (next !== prev) {
        listener(next, prev);
        prev = next;
      }
      void prevState;
    });
  };

  return api;
}