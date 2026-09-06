import type { ReactNode } from 'react';
import { Navigate } from 'react-router-dom';
import { useAdmin } from './admin-context';

/**
 * Защищает страницы админки: без валидного токена в tokenStore — редирект
 * на /login. Читает store напрямую (не состояние), сохраняя контракт
 * tokenStore как zustand-подобного slice.
 */
export function RequireAuth({ children }: { children: ReactNode }) {
  const { tokenStore } = useAdmin();
  if (tokenStore.getState().token == null) {
    return <Navigate to="/login" replace />;
  }
  return <>{children}</>;
}