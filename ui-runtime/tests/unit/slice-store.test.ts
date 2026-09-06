import { describe, expect, it, vi } from 'vitest';
import { act, renderHook } from '@testing-library/react';
import { createSliceStore } from '../../src/core/slice-store';
import { useSelector } from '../../src/react/use-selector';

interface Counter {
  count: number;
  label: string;
}

describe('createSliceStore / useSelector', () => {
  it('создаётся с initial-состоянием', () => {
    const store = createSliceStore<Counter>({ count: 0, label: 'x' });
    expect(store.getState()).toEqual({ count: 0, label: 'x' });
  });

  it('setState(partial) мержит shallow, не трогая остальные срезы', () => {
    const store = createSliceStore<Counter>({ count: 0, label: 'x' });
    store.setState({ count: 5 });
    expect(store.getState()).toEqual({ count: 5, label: 'x' });
  });

  it('setState(updater) получает prev и возвращает патч', () => {
    const store = createSliceStore<Counter>({ count: 0, label: 'x' });
    store.setState((prev) => ({ count: prev.count + 1 }));
    store.setState((prev) => ({ count: prev.count + 1 }));
    expect(store.getState().count).toBe(2);
  });

  it('subscribe уведомляет (state, prev) и возвращает отписку', () => {
    const store = createSliceStore<Counter>({ count: 0, label: 'x' });
    const fn = vi.fn();
    const off = store.subscribe(fn);
    store.setState({ count: 1 });
    expect(fn).toHaveBeenCalledTimes(1);
    expect(fn.mock.calls[0][0].count).toBe(1);
    expect(fn.mock.calls[0][1].count).toBe(0);
    off();
    store.setState({ count: 2 });
    expect(fn).toHaveBeenCalledTimes(1);
  });

  it('subscribeSlice вызывает listener только при ===-изменении выбранного среза', () => {
    const store = createSliceStore<Counter>({ count: 0, label: 'x' });
    const fn = vi.fn();
    store.subscribeSlice((s) => s.count, fn);
    store.setState({ count: 1 });
    expect(fn).toHaveBeenCalledTimes(1);
    store.setState({ label: 'y' });
    expect(fn).toHaveBeenCalledTimes(1);
  });

  it('useSelector перерисовывается только при изменении выбранного среза', () => {
    const store = createSliceStore<Counter>({ count: 0, label: 'x' });
    const { result } = renderHook(() => useSelector(store, (s) => s.count));

    expect(result.current).toBe(0);

    act(() => store.setState({ label: 'y' }));
    expect(result.current).toBe(0);

    act(() => store.setState({ count: 42 }));
    expect(result.current).toBe(42);
  });

  it('useSelector отписывается при размонтировании', () => {
    const store = createSliceStore<Counter>({ count: 0, label: 'x' });
    const occurrences = vi.fn();
    store.subscribe(occurrences);

    const { unmount } = renderHook(() => useSelector(store, (s) => s.count));
    const before = occurrences.mock.calls.length;
    unmount();

    act(() => store.setState({ count: 1 }));
    expect(occurrences.mock.calls.length).toBe(before + 1);
  });
});