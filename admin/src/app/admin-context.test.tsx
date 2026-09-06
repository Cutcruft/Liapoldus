import { describe, expect, it } from 'vitest';
import { renderHook } from '@testing-library/react';
import type { ReactNode } from 'react';
import { AdminProvider, useAdmin } from './admin-context';
import { createAdminApi, createTokenStore } from '../runtime';

function wrapper({ children }: { children: ReactNode }) {
  const tokenStore = createTokenStore('t');
  const api = createAdminApi({ baseUrl: '', getToken: () => null });
  return <AdminProvider api={api} tokenStore={tokenStore}>{children}</AdminProvider>;
}

describe('admin-context', () => {
  it('useAdmin вне провайдера бросает', () => {
    expect(() => renderHook(() => useAdmin())).toThrow();
  });

  it('useAdmin внутри провайдера отдаёт api/store/t', () => {
    const { result } = renderHook(() => useAdmin(), { wrapper });
    expect(result.current.api).toBeTruthy();
    expect(result.current.tokenStore.getState().token).toBe('t');
    expect(result.current.t('nav.sites')).toBe('Сайты');
  });
});