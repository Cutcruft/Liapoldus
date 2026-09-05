import { useCallback, useEffect, useMemo, useState } from 'react';
import { useStore } from 'zustand';
import type { RuntimeState } from '../core/store';
import { createRuntimeStore } from '../core/store';
import type { ContentData } from '../types/content';
import type { ResolvedRoute } from '../types/descriptor';
import type { FieldSchema } from '../types/form';
import type { TreeDeclaration } from '../types/tree';
import { useRuntime, useRuntimeContext } from './context';

/** Пустой стор — рендер вне готового RuntimeProvider не падает. */
const fallbackStore = createRuntimeStore();

/** Реактивность на zustand-сторе (useStore + селектор по срезу). */
export function useRuntimeRaw<T>(selector: (s: RuntimeState) => T): T {
  const { runtime } = useRuntimeContext();
  return useStore(runtime?.store ?? fallbackStore, selector);
}

/** useContent(id): значение из store.content по id; отсутствует → undefined. */
export function useContent<T = ContentData>(id: string): T | undefined {
  return useRuntimeRaw((s) => s.content[id] as T | undefined) ?? (undefined as T | undefined);
}

export interface QueryStatus<T> {
  status: 'idle' | 'pending' | 'success' | 'error';
  data?: T;
  error?: Error;
}

export interface UseQueryOptions {
  /** short polling: cron-расписание; тики обновляют result без ручного re-рендера */
  schedule?: string;
}

/** useQuery: query по operationId; повторный эффект при смене input (§17#3-#5). */
export function useQuery<T = unknown>(operationId: string, input?: Record<string, unknown>, opts?: UseQueryOptions): QueryStatus<T> {
  const runtime = useRuntime();
  const [result, setResult] = useState<QueryStatus<T>>({ status: 'idle' });
  const inputKey = JSON.stringify(input ?? {});

  useEffect(() => {
    if (opts?.schedule) {
      let active = true;
      const unsubscribe = runtime.client.poll<T>(
        operationId,
        (data) => {
          if (active) setResult({ status: 'success', data });
        },
        { schedule: opts.schedule, input },
      );
      return () => {
        active = false;
        unsubscribe();
      };
    }

    let active = true;
    setResult((prev) => (prev.status === 'success' || prev.status === 'error' ? { status: 'pending' } : prev));
    runtime.client
      .query<Record<string, unknown> | undefined, T>(operationId, input)
      .then((data) => {
        if (active) setResult({ status: 'success', data });
      })
      .catch((e: unknown) => {
        if (active) setResult({ status: 'error', error: e instanceof Error ? e : new Error(String(e)) });
      });
    return () => {
      active = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [operationId, inputKey, opts?.schedule]);

  return result;
}

export interface MutationStatus<T> {
  status: 'idle' | 'pending' | 'success' | 'error';
  data?: T;
  error?: Error;
}

export interface UseMutationResult<T> extends MutationStatus<T> {
  mutate(input?: Record<string, unknown>): Promise<T | undefined>;
  reset(): void;
}

/** useMutation: status idle→pending→success/error (§17#6). */
export function useMutation<T = unknown>(operationId: string): UseMutationResult<T> {
  const runtime = useRuntime();
  const [state, setState] = useState<MutationStatus<T>>({ status: 'idle' });

  const mutate = useCallback(
    async (input?: Record<string, unknown>): Promise<T | undefined> => {
      setState({ status: 'pending' });
      try {
        const data = await runtime.client.mutate<Record<string, unknown> | undefined, T>(operationId, input);
        setState({ status: 'success', data });
        return data;
      } catch (e) {
        const error = e instanceof Error ? e : new Error(String(e));
        setState({ status: 'error', error });
        return undefined;
      }
    },
    [runtime, operationId],
  );

  const reset = useCallback(() => setState({ status: 'idle' }), []);

  return useMemo(() => ({ ...state, mutate, reset }), [state, mutate, reset]);
}

/** useDesignToken(name): css-переменная из store.tokens (§17#7). */
export function useDesignToken(name: string): string | undefined {
  return useRuntimeRaw((s) => s.tokens[name]);
}

/** useRoute(): текущий ResolvedRoute (null до навигации) (§17#8). */
export function useRoute(): ResolvedRoute | null {
  return useRuntimeRaw((s) => s.route) ?? null;
}

/** useTree(): текущая декларация дерева (§17#9). */
export function useTree(): TreeDeclaration | null {
  return useRuntimeRaw((s) => s.tree) ?? null;
}

/** useReady(): true после boot (§17#10). */
export function useReady(): boolean {
  return useRuntimeContext().ready;
}

/** useCurrentLocale(): текущий язык из стора (§17#11). */
export function useCurrentLocale(): string {
  return useRuntimeRaw((s) => s.locale) || '';
}

export interface AssetRefInput {
  assetId: string;
  variant?: string;
}

/** useAsset({assetId, variant}): URL варианта из store.assets (default 'master'); неизвестный ассет → undefined (§17#12). */
export function useAsset(ref: AssetRefInput): string | undefined {
  return useRuntimeRaw((s) => {
    const meta = s.assets[ref.assetId];
    if (!meta) return undefined;
    const variant = ref.variant ?? 'master';
    return meta.variants.find((v) => v.name === variant)?.url;
  });
}

/** useT(): UI-строки (коллекция `strings`) под текущим locale; re-render на смене locale/content (§17#11). */
export function useT(): (key: string, params?: Record<string, string | number>) => string {
  const runtime = useRuntime();
  useCurrentLocale();
  useRuntimeRaw((s) => s.content);
  return useCallback((key: string, params?: Record<string, string | number>) => runtime.i18n.t(key, params), [runtime]);
}

export interface FieldControl {
  name: string;
  value: string;
  onChange: (e: { target: { value: unknown } }) => void;
}

export interface UseFormResult<T extends Record<string, unknown> = Record<string, unknown>> {
  status: 'idle' | 'loading' | 'error' | 'submitting' | 'submitted';
  values: T;
  errors: Record<string, string[]>;
  fields: FieldSchema[];
  register(name: string): FieldControl;
  handleSubmit(): Promise<void>;
  reset(): void;
}

function errorMap(errors: Array<{ fieldId: string; message?: string }>): Record<string, string[]> {
  const out: Record<string, string[]> = {};
  for (const e of errors) (out[e.fieldId] ??= []).push(e.message ?? e.fieldId);
  return out;
}

/** useForm(formId): значения/ошибки/статус из стора + register/handleSubmit (клиентская валидация, §17#13). */
export function useForm<T extends Record<string, unknown> = Record<string, unknown>>(formId: string): UseFormResult<T> {
  const runtime = useRuntime();
  const snap = useRuntimeRaw((s) => s.forms[formId]);

  useEffect(() => {
    let active = true;
    void runtime.forms.load(formId).catch(() => {
      if (active) runtime.store.getState().setFormState(formId, { status: 'error' });
    });
    return () => {
      active = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [formId, runtime]);

  const register = useCallback<UseFormResult['register']>(
    (name) => {
      const raw = snap?.values[name];
      return {
        name,
        value: typeof raw === 'string' || typeof raw === 'number' ? String(raw) : '',
        onChange: (e: { target: { value: unknown } }) => {
          runtime.store.getState().setFormState(formId, { values: { ...(snap?.values ?? {}), [name]: e.target.value } });
        },
      };
    },
    [runtime, formId, snap],
  );

  const values = useMemo(() => (snap?.values ?? {}) as T, [snap]);

  const handleSubmit = useCallback(async () => {
    if (!snap?.definition) {
      await runtime.forms.load(formId);
    }
    const allValues = (runtime.store.getState().forms[formId]?.values ?? {}) as Record<string, unknown>;
    const validated = runtime.forms.validate(formId, allValues);
    if (!validated.valid) {
      runtime.store.getState().setFormState(formId, { status: 'error', errors: errorMap(validated.errors) });
      return; // submit-запрос при невалидной форме не выполняется
    }
    await runtime.forms.submit(formId, allValues);
  }, [runtime, formId, snap?.definition]);

  const reset = useCallback(() => runtime.forms.reset(formId), [runtime, formId]);

  return useMemo<UseFormResult<T>>(
    () => ({
      status: snap?.status ?? 'idle',
      values,
      errors: snap?.errors ?? {},
      fields: snap?.definition?.fields ?? [],
      register,
      handleSubmit,
      reset,
    }),
    [snap, values, register, handleSubmit, reset],
  );
}