import { useCallback, useEffect, useMemo, useState } from 'react';
import { runOperation, type OperationKind, type AdminApiError } from '../runtime';
import { useAdmin } from './admin-context';

export type UseOperationState<T> =
  | { status: 'loading' }
  | { status: 'success'; data: T }
  | { status: 'error'; error: AdminApiError; detail: string };

export interface UseOperationResult<T> {
  state: UseOperationState<T>;
  reload: () => void;
}

/**
 * Выполняет операцию admin API при монтировании/смене args и возвращает результат.
 * Пере-запрос — через reload(); нормализация данных — в колбэке mapData (по умолчанию data as T).
 */
export function useOperation<T = unknown>(
  kind: OperationKind,
  args: Record<string, unknown>,
  mapData?: (data: unknown) => T,
): UseOperationResult<T> {
  const { api, t } = useAdmin();
  const [data, setData] = useState<T | undefined>(undefined);
  const [status, setStatus] = useState<'loading' | 'success' | 'error'>('loading');
  const [error, setError] = useState<{ error?: AdminApiError; detail: string }>({ detail: '' });
  const [nonce, setNonce] = useState(0);

  const argsKey = JSON.stringify(args ?? {});
  // Поставщик данных, чтобы эффект не зависел от identity mapData/args.
  const normalize = useCallback(
    (raw: unknown) => (mapData ? mapData(raw) : (raw as T)),
    [argsKey],
  );

  useEffect(() => {
    let cancelled = false;
    setStatus('loading');
    runOperation(api, kind, args, t).then((res) => {
      if (cancelled) return;
      if (res.ok) {
        setData(normalize(res.data));
        setStatus('success');
      } else {
        setError({ error: res.error, detail: res.detail });
        setStatus('error');
      }
    });
    return () => {
      cancelled = true;
    };
  }, [api, t, kind, argsKey, nonce, normalize]);

  const reload = useCallback(() => setNonce((n) => n + 1), []);

  return useMemo(() => {
    if (status === 'loading') return { state: { status: 'loading' }, reload };
    if (status === 'error')
      return {
        state: {
          status: 'error',
          error: error.error ?? { kind: 'http', status: undefined, message: error.detail },
          detail: error.detail,
        },
        reload,
      };
    return { state: { status: 'success', data: data as T }, reload };
  }, [status, error, data, reload]);
}