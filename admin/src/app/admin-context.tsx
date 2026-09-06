import { createContext, useContext, useMemo, type ReactNode } from 'react';
import { makeTranslate, STRINGS, type Translate, type AdminApi, type AdminAuthState, type SliceStore, type AdminStringKey } from '../runtime';

export interface AdminContextValue {
  api: AdminApi;
  tokenStore: SliceStore<AdminAuthState>;
  t: Translate;
}

export const AdminContext = createContext<AdminContextValue | null>(null);

export const ADMIN_TOKEN_KEY = 'liapoldus.admin.token';

export function AdminProvider({
  api,
  tokenStore,
  children,
}: {
  api: AdminApi;
  tokenStore: SliceStore<AdminAuthState>;
  children: ReactNode;
}) {
  const t = useMemo<Translate>(() => makeTranslate(STRINGS), []);
  const value = useMemo(() => ({ api, tokenStore, t }), [api, tokenStore, t]);
  return <AdminContext.Provider value={value}>{children}</AdminContext.Provider>;
}

export function useAdmin(): AdminContextValue {
  const ctx = useContext(AdminContext);
  if (!ctx) throw new Error('useAdmin должен использоваться внутри AdminProvider');
  return ctx;
}

export function useStrings(key: AdminStringKey): string {
  return useAdmin().t(key);
}