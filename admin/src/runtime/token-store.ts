import { createSliceStore, type SliceStore } from '@liapoldus/ui-runtime';
import type { AdminApi } from './api';

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

/**
 * Проверяет токен на сервере (`POST /api/auth/validate`). Сервер открыт
 * (без конфигурации AdminToken) — принимается любой непустой токен.
 */
export async function validateToken(api: AdminApi, token: string): Promise<boolean> {
  if (!token.trim()) return false;
  const res = await api.request('POST', '/api/auth/validate', { token });
  if (!res.ok) return false;
  const body = res.body as { valid?: unknown } | null;
  return typeof body === 'object' && body !== null && body.valid === true;
}

export type { SliceStore };