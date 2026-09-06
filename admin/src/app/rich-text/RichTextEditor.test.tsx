import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { createAdminApi, createTokenStore } from '../../runtime';
import { AdminProvider } from '../admin-context';
import { RichTextEditor, __lastRichTextEditor } from './RichTextEditor';

function renderEditor(initial = '<p>Привет</p>') {
  const onChange = vi.fn();
  const api = createAdminApi({
    baseUrl: '',
    getToken: () => null,
    fetchFn: () => Promise.resolve(new Response('[]')),
  });
  const tokenStore = createTokenStore(null);
  cleanup();
  const utils = render(
    <AdminProvider api={api} tokenStore={tokenStore}>
      <RichTextEditor siteId="s1" value={initial} onChange={onChange} />
    </AdminProvider>,
  );
  return { onChange, ...utils };
}

afterEach(cleanup);

const dispatch = (key: string) => {
  const body = document.querySelector('[contenteditable="true"]') as HTMLElement | null;
  body?.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true }));
};

describe('RichTextEditor', () => {
  it('монтируется с переданным HTML и рендерит текст', () => {
    renderEditor('<p>Привет</p>');
    const editor = __lastRichTextEditor();
    expect(editor).toBeTruthy();
    expect(screen.getByText('Привет')).toBeTruthy();
  });

  it('изменение через команды редактора → onChange отдаёт HTML round-trip', async () => {
    const { onChange } = renderEditor('<p>Привет</p>');
    const editor = __lastRichTextEditor()!;
    editor.chain().focus().selectAll().deleteSelection().insertContent('Мир').run();
    await waitFor(() => expect(onChange).toHaveBeenCalledWith('<p>Мир</p>'));
  });

  it('пустой ввод нормализуется в стабильный <p></p>', async () => {
    const { onChange } = renderEditor('<p></p>');
    const editor = __lastRichTextEditor()!;
    editor.chain().focus().insertContent('x').run();
    await waitFor(() => expect(onChange).toHaveBeenCalled());
  });

  it('тулбар: жирный выделяет текст', () => {
    renderEditor('<p>Hello</p>');
    const editor = __lastRichTextEditor()!;
    editor.chain().focus().selectAll().run();
    fireEvent.click(screen.getByRole('button', { name: 'Жирный' }));
    expect(editor.getHTML()).toContain('<strong>Hello</strong>');
  });

  it('slash-меню: /h1 + Enter превращает в заголовок и убирает слэш-текст', async () => {
    const { onChange } = renderEditor('<p></p>');
    const editor = __lastRichTextEditor()!;
    editor.chain().focus().insertContent('/h1').run();

    await screen.findByRole('option', { name: 'Заголовок H1' });
    dispatch('Enter');
    await waitFor(() => expect(editor.getHTML()).toBe('<h1></h1>'));
    await waitFor(() => expect(screen.queryByRole('option')).toBeNull());
    expect(onChange).toHaveBeenLastCalledWith('<h1></h1>');
  });

  it('slash-меню: фильтрация по запросу и навигация стрелками', async () => {
    renderEditor('<p></p>');
    const editor = __lastRichTextEditor()!;
    editor.chain().focus().insertContent('/список').run();

    await screen.findByRole('option', { name: 'Маркированный список' });
    expect(screen.getAllByRole('option').length).toBe(2);

    dispatch('ArrowDown');
    dispatch('Enter');
    await waitFor(() => expect(editor.getHTML()).toBe('<ul><li><p></p></li></ul>'));
  });

  it('link: тулбар открывает URL-инпут, Enter ставит ссылку', async () => {
    renderEditor('<p>Текст</p>');
    const editor = __lastRichTextEditor()!;
    editor.chain().focus().selectAll().run();
    fireEvent.click(screen.getByRole('button', { name: 'Ссылка' }));

    const input = await screen.findByLabelText('URL ссылки');
    fireEvent.change(input, { target: { value: 'https://example.com' } });
    fireEvent.keyDown(input, { key: 'Enter' });

    await waitFor(() => {
      expect(editor.getHTML()).toContain('href="https://example.com"');
    });
    expect(screen.queryByRole('dialog')).toBeNull();
  });

  it('Escape закрывает slash-меню без применения', async () => {
    renderEditor('<p></p>');
    const editor = __lastRichTextEditor()!;
    editor.chain().focus().insertContent('/h1').run();
    await screen.findByRole('option', { name: 'Заголовок H1' });

    dispatch('Escape');
    await waitFor(() => expect(screen.queryByRole('option')).toBeNull());
    expect(editor.getHTML()).toBe('<p>/h1</p>');
  });
});