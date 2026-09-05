import { createContext, useContext, useEffect, useState, type ReactNode } from 'react';
import type { BootRuntime } from '../core/boot';

/**
 * Контекст runtime: BootRuntime публикуется через RuntimeProvider.
 * Пока boot в полёте (Promise) — ready=false, children не отдаются.
 */
export interface RuntimeContextState {
  runtime?: BootRuntime;
  ready: boolean;
  error?: Error;
}

export interface RuntimeProviderProps {
  /** уже забоченный runtime или промис boot() */
  runtime: BootRuntime | Promise<BootRuntime>;
  children: ReactNode;
}

const RuntimeContext = createContext<RuntimeContextState>({ ready: false });

function isBootRuntime(value: BootRuntime | Promise<BootRuntime>): value is BootRuntime {
  return typeof value === 'object' && value !== null && typeof (value as { then?: unknown }).then !== 'function';
}

export function RuntimeProvider({ runtime, children }: RuntimeProviderProps) {
  const resolved = isBootRuntime(runtime);
  // Уже забоченный runtime → синхронный первый рендер (без гонки с act()/микротасками).
  const [state, setState] = useState<RuntimeContextState>(() =>
    resolved ? { runtime, ready: true } : { ready: false },
  );

  useEffect(() => {
    if (resolved) return;
    let active = true;
    Promise.resolve(runtime)
      .then((r) => {
        if (!active) return;
        r.store.getState().setReady(true);
        setState({ runtime: r, ready: true });
      })
      .catch((e: unknown) => {
        if (!active) return;
        setState({ ready: false, error: e instanceof Error ? e : new Error(String(e)) });
      });
    return () => {
      active = false;
    };
  }, [runtime, resolved]);

  // пока runtime готовится — children не рендерим (хуки требуют готового runtime)
  return <RuntimeContext.Provider value={state}>{state.ready ? children : null}</RuntimeContext.Provider>;
}

export function useRuntimeContext(): RuntimeContextState {
  return useContext(RuntimeContext);
}

/** Бросает, если runtime ещё не готов (рендер вне RuntimeProvider/до boot). */
export function useRuntime(): BootRuntime {
  const ctx = useContext(RuntimeContext);
  if (!ctx.runtime) {
    throw new Error('useRuntime: RuntimeProvider не готов (дождитесь boot)');
  }
  return ctx.runtime;
}

export { RuntimeContext };