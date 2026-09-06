import { createSliceStore, type SliceStore } from '@liapoldus/ui-runtime';

export interface AdminAuthState {
  token: string | null;
}

/**
 * Хранение admin-токена: slice-store (zustand-обёртка ui-runtime), а не zustand.
 * Локальное состояние токена — на `localStorage` в UI-прослойке (M1).
 */
export function createTokenStore(initialToken: string | null = null): SliceStore<AdminAuthState> {
  return createSliceStore<AdminAuthState>({ token: initialToken });
}

export type { SliceStore };