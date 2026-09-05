import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it } from 'vitest';
import type { ReactNode } from 'react';
import type { BootRuntime } from '../../src/core/boot';
import {
  RuntimeProvider,
  useAsset,
  useContent,
  useCurrentLocale,
  useDesignToken,
  useForm,
  useMutation,
  useQuery,
  useReady,
  useRoute,
  useT,
  useTree,
} from '../../src/react';
import { bootFromContract, buildContract, until, feedOp, routeArticle } from './react-harness';
import { resetFakes, type FetchCall } from './helpers';

function Scaffold({ runtime, children }: { runtime: BootRuntime; children: ReactNode }) {
  return <RuntimeProvider runtime={runtime}>{children}</RuntimeProvider>;
}

function feedsCalls(calls: FetchCall[]): FetchCall[] {
  return calls.filter((c) => new URL(c.url).pathname.endsWith('/feeds'));
}

describe('17. react-hooks', () => {
  beforeEach(() => {
    resetFakes();
  });

  it('1. useContent(id) → значение из стора', async () => {
    const { runtime } = await bootFromContract();
    runtime.store.getState().setContent({ post: { title: 'Привет' } });

    function View() {
      const title = useContent<{ title: string }>('post')?.title;
      return <div data-testid="c">{title}</div>;
    }

    render(
      <Scaffold runtime={runtime}>
        <View />
      </Scaffold>,
    );
    await waitFor(() => expect(screen.getByTestId('c').textContent).toContain('Привет'));
  });

  it('2. useContent(id) при отсутствии → undefined (компонент не падает)', async () => {
    const { runtime } = await bootFromContract();

    function View() {
      const value = useContent('nope');
      return <div data-testid="c">{value === undefined ? 'undefined' : 'found'}</div>;
    }

    render(
      <Scaffold runtime={runtime}>
        <View />
      </Scaffold>,
    );
    await waitFor(() => expect(screen.getByTestId('c').textContent).toBe('undefined'));
  });

  it('3. useQuery: успех → результат; ошибка → статус error', async () => {
    let fail = false;
    const { runtime } = await bootFromContract(
      { operations: [feedOp(false)] },
      (call) => {
        const u = new URL(call.url);
        if (u.pathname.endsWith('/feeds')) {
          if (fail) return new Error('boom');
          return { state: 'ok', items: [{ id: 'n1' }] };
        }
        return buildContract({ operations: [feedOp(false)] });
      },
    );

function View() {
      const q = useQuery<{ items: Array<{ id: string }> }>('feed.list', { siteId: 'acme' });
      return (
        <div data-testid="q">
          {q.status === 'success' ? (q.data?.items ?? []).map((i) => i.id).join(',') : q.status}
        </div>
      );
    }

    const { rerender } = render(
      <Scaffold runtime={runtime}>
        <View />
      </Scaffold>,
    );
    await waitFor(() => expect(screen.getByTestId('q').textContent).toContain('n1'));

    fail = true;
    rerender(
      <Scaffold runtime={runtime}>
        <View key="err" />
      </Scaffold>,
    );
    await waitFor(() => expect(screen.getByTestId('q').textContent).toBe('error'));
  });

  it('4. useQuery: смена input → повторный запрос (2 вызова fetch)', async () => {
    let page = 1;
    const { runtime, calls } = await bootFromContract(
      { operations: [feedOp(false)] },
      (call) => {
        const u = new URL(call.url);
        if (u.pathname.endsWith('/feeds')) return { state: 'ok', items: [{ id: `n${page}` }] };
        return buildContract({ operations: [feedOp(false)] });
      },
    );

    function View() {
      const q = useQuery<{ items: Array<{ id: string }> }>('feed.list', { siteId: 'acme', page });
      const data = q.status === 'success' ? (q.data?.items ?? []).map((i) => i.id).join(',') : q.status;
      return <div data-testid="q">{data}</div>;
    }

    const { rerender } = render(
      <Scaffold runtime={runtime}>
        <View />
      </Scaffold>,
    );
    await waitFor(() => expect(screen.getByTestId('q').textContent).toContain('n1'));
    expect(feedsCalls(calls)).toHaveLength(1);

    page = 2;
    rerender(
      <Scaffold runtime={runtime}>
        <View />
      </Scaffold>,
    );
    await waitFor(() => expect(screen.getByTestId('q').textContent).toContain('n2'));
    const fc = feedsCalls(calls);
    expect(fc).toHaveLength(2);
    expect(new URL(fc[1].url).searchParams.get('page')).toBe('2');
  });

  it('5. useQuery со schedule → short polling: тики обновляют результат без ручного re-рендера', async () => {
    let tick = 0;
    const { runtime } = await bootFromContract({}, (call) => {
      const u = new URL(call.url);
      if (u.pathname.endsWith('/feeds')) {
        tick += 1;
        return { state: 'ok', items: [{ id: `n${tick}` }] };
      }
      return buildContract();
    });

    function View() {
      const q = useQuery<{ items: Array<{ id: string }> }>('feed.list', { siteId: 'acme' }, { schedule: '* * * * * *' });
      const data = q.status === 'success' ? (q.data?.items ?? []).map((i) => i.id).join(',') : q.status;
      return <div data-testid="q">{data}</div>;
    }

    render(
      <Scaffold runtime={runtime}>
        <View />
      </Scaffold>,
    );
    await waitFor(() => expect(screen.getByTestId('q').textContent).toMatch(/n\d/));
    await until(() => screen.getByTestId('q').textContent === 'n2');
    expect(screen.getByTestId('q').textContent).toBe('n2');
  });

  it('6. useMutation: mutate() → запрос; idle→pending→success', async () => {
    const likeOp = {
      id: 'like.post',
      typeOp: 'mutation',
      providerId: 'cms',
      method: 'POST',
      path: '/api/posts/like',
      params: { in: 'query', fields: { postId: { required: true } } },
      cache: 'disabled',
    };
    const { runtime, calls } = await bootFromContract(
      { operations: [feedOp(false), likeOp] },
      (call) => {
        const u = new URL(call.url);
        if (u.pathname.endsWith('/like')) return { state: 'ok', likes: 2 };
        return buildContract({ operations: [feedOp(false), likeOp] });
      },
    );

    function View() {
      const m = useMutation<{ likes: number }>('like.post');
      return (
        <div>
          <span data-testid="status">{m.status}</span>
          <button data-testid="go" onClick={() => void m.mutate({ postId: 'p1' })}>
            like
          </button>
        </div>
      );
    }

    render(
      <Scaffold runtime={runtime}>
        <View />
      </Scaffold>,
    );
    await waitFor(() => expect(screen.getByTestId('status').textContent).toBe('idle'));
    fireEvent.click(screen.getByTestId('go'));
    expect(screen.getByTestId('status').textContent).toBe('pending');
    await waitFor(() => expect(screen.getByTestId('status').textContent).toBe('success'));
    expect(calls.some((c) => new URL(c.url).pathname.endsWith('/like') && (c.init.method ?? 'GET').toUpperCase() === 'POST')).toBe(true);
  });

  it('7. useDesignToken(name) → css-переменная из стора', async () => {
    const { runtime } = await bootFromContract();
    function View() {
      return <div data-testid="t">{useDesignToken('--color-bg')}</div>;
    }
    render(
      <Scaffold runtime={runtime}>
        <View />
      </Scaffold>,
    );
    await waitFor(() => expect(screen.getByTestId('t').textContent).toBe('#ffffff'));
  });

  it('8. useRoute(): роут из стора; null до навигации', async () => {
    const { runtime } = await bootFromContract({ routes: [routeArticle()] });
    function View() {
      const r = useRoute();
      return <div data-testid="r">{r === null ? 'null' : r.route.id}</div>;
    }
    render(
      <Scaffold runtime={runtime}>
        <View />
      </Scaffold>,
    );
    await waitFor(() => expect(screen.getByTestId('r').textContent).toBe('null'));
    act(() => runtime.router.navigate('/articles/42'));
    await waitFor(() => expect(screen.getByTestId('r').textContent).toBe('article'));
  });

  it('9. useTree(): декларация из стора', async () => {
    const { runtime } = await bootFromContract();
    runtime.store.getState().setTree({
      snapshotId: 's1',
      root: { instanceId: 'root', definitionId: 'page.home', props: {}, bindings: [], children: [] },
    });
    function View() {
      return <div data-testid="tr">{useTree()?.root.instanceId ?? 'none'}</div>;
    }
    render(
      <Scaffold runtime={runtime}>
        <View />
      </Scaffold>,
    );
    await waitFor(() => expect(screen.getByTestId('tr').textContent).toBe('root'));
  });

  it('10. useReady(): true после boot', async () => {
    const { runtime } = await bootFromContract();
    function View() {
      return <div data-testid="rd">{useReady() ? 'ready' : 'no'}</div>;
    }
    render(
      <Scaffold runtime={runtime}>
        <View />
      </Scaffold>,
    );
    await waitFor(() => expect(screen.getByTestId('rd').textContent).toBe('ready'));
  });

  it('11. useCurrentLocale() + useT() читают строки из контента под locale', async () => {
    const { runtime } = await bootFromContract({ locale: 'ru' });
    runtime.store.getState().setContent({
      strings: { greeting: 'Здравствуйте, {name}', bye: 'До свидания' },
    });
    function View() {
      const t = useT();
      return (
        <div>
          <span data-testid="loc">{useCurrentLocale()}</span>
          <span data-testid="str">{t('strings.greeting', { name: 'Анна' })}</span>
        </div>
      );
    }
    render(
      <Scaffold runtime={runtime}>
        <View />
      </Scaffold>,
    );
    await waitFor(() => expect(screen.getByTestId('loc').textContent).toBe('ru'));
    await waitFor(() => expect(screen.getByTestId('str').textContent).toBe('Здравствуйте, Анна'));
  });

  it('12. useAsset({assetId, variant}) → URL варианта; неизвестный assetId → undefined', async () => {
    const { runtime } = await bootFromContract();
    runtime.store.getState().setAssets({
      'asset-1': {
        id: 'asset-1',
        name: 'pic',
        type: 'image',
        size: 3,
        variants: [
          { name: 'master', url: 'https://cdn.test/pic.png' },
          { name: 'thumb', url: 'https://cdn.test/pic-thumb.png' },
        ],
      },
    });
    function View() {
      const a = useAsset({ assetId: 'asset-1', variant: 'thumb' });
      const b = useAsset({ assetId: 'missing' });
      return (
        <div>
          <span data-testid="a">{a ?? 'none'}</span>
          <span data-testid="b">{b === undefined ? 'undefined' : b}</span>
        </div>
      );
    }
    render(
      <Scaffold runtime={runtime}>
        <View />
      </Scaffold>,
    );
    await waitFor(() =>
      expect(screen.getByTestId('a').textContent).toBe('https://cdn.test/pic-thumb.png'),
    );
    await waitFor(() => expect(screen.getByTestId('b').textContent).toBe('undefined'));
  });

  it('13. useForm: валидация клиентская; submit при невалидной форме не выполняется', async () => {
    const formDef = {
      id: 'contact',
      name: 'Контакты',
      fields: [
        { name: 'email', type: 'email', label: 'Email', required: true, rules: [{ id: 'minLength', value: 3 }] },
      ],
      submit: { target: 'form.submit' },
    };
    const { runtime, calls } = await bootFromContract(
      { operations: [feedOp(false)] },
      (call) => {
        const u = new URL(call.url, 'http://localhost');
        if (u.pathname === '/api/forms/contact') return formDef;
        if (u.pathname.endsWith('/submissions')) return { submissionId: 's1', state: 'ok' };
        return buildContract({ operations: [feedOp(false)] });
      },
    );

    function View() {
      const f = useForm<{ email?: string }>('contact');
      return (
        <div>
          <input data-testid="email" {...f.register('email')} />
          <button data-testid="submit" onClick={() => void f.handleSubmit()}>
            {f.status}
          </button>
          <span data-testid="errors">{JSON.stringify(f.errors)}</span>
          <span data-testid="status2">{f.status}</span>
        </div>
      );
    }

    render(
      <Scaffold runtime={runtime}>
        <View />
      </Scaffold>,
    );
    await waitFor(() => expect(screen.getByTestId('status2').textContent).not.toBe('loading'));

    // невалидный email → submit не отправляется, ошибка в сторе
    fireEvent.change(screen.getByTestId('email'), { target: { value: 'not-an-email' } });
    fireEvent.click(screen.getByTestId('submit'));
    await waitFor(() => expect(screen.getByTestId('errors').textContent).toContain('email'));
    expect(calls.filter((c) => c.url.includes('/submissions'))).toHaveLength(0);

    // валидный email → submit выполнен, status submitted
    fireEvent.change(screen.getByTestId('email'), { target: { value: 'a@b.co' } });
    fireEvent.click(screen.getByTestId('submit'));
    await waitFor(() => expect(screen.getByTestId('status2').textContent).toBe('submitted'));
    expect(calls.filter((c) => c.url.includes('/submissions'))).toHaveLength(1);
  });

  it('14. подписка на content → событие в сторе обновляет компонент без remount', async () => {
    const { runtime } = await bootFromContract();
    runtime.store.getState().setContent({ post: { title: 'v1' } });
    function View() {
      const title = useContent<{ title: string }>('post')?.title;
      dataMounts.current += 1;
      return (
        <div data-testid="post" data-mounts={dataMounts.current}>
          {title}
        </div>
      );
    }
    render(
      <Scaffold runtime={runtime}>
        <View />
      </Scaffold>,
    );
    await waitFor(() => expect(screen.getByTestId('post').textContent).toBe('v1'));
    const before = screen.getByTestId('post');

    act(() => runtime.store.getState().setContent({ post: { title: 'v2' } }));
    await waitFor(() => expect(before.textContent).toBe('v2'));
    expect(screen.getByTestId('post')).toBe(before);
  });
});

const dataMounts: { current: number } = { current: 0 };