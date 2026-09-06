import { useEffect, useMemo, useRef, useState } from 'react';
import { useEditor, EditorContent, type Editor } from '@tiptap/react';
import { Extension } from '@tiptap/core';
import StarterKit from '@tiptap/starter-kit';
import Link from '@tiptap/extension-link';
import Image from '@tiptap/extension-image';
import { useAdmin } from '../admin-context';
import { AssetPicker } from '../assets/AssetPicker';
import { normalizeRichHtml } from './html';
import {
  getTextBeforeCursor,
  labelKeyFor,
  matchSlashCommands,
  runSlashCommand,
  runInlineMark,
  TOOLBAR_ORDER,
  type SlashCommandKind,
  type ToolbarKind,
} from './commands';

const TOOL_CLASS =
  'rounded border border-neutral-200 px-1.5 py-0.5 text-xs leading-none text-neutral-600 hover:border-blue-400 hover:text-blue-600 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-blue-400';

// Тестовый хук: последний смонтированный редактор (для интеграционных тестов страницы).
let lastEditor: Editor | null = null;
export function __lastRichTextEditor(): Editor | null {
  return lastEditor;
}

/** Единый кеймэп: стрелки/Enter/Escape отдают приоритет открытому slash-меню. */
function createSlashKeymap(keymapRef: { current: (key: string) => boolean }) {
  return Extension.create({
    name: 'slashKeymap',
    addKeyboardShortcuts() {
      return {
        ArrowDown: () => keymapRef.current('ArrowDown'),
        ArrowUp: () => keymapRef.current('ArrowUp'),
        Escape: () => keymapRef.current('Escape'),
        Enter: () => keymapRef.current('Enter'),
      };
    },
  });
}

const EDITOR_CLASS =
  'prose prose-sm max-w-none focus:outline-none [&_p]:my-1 [&_h1]:my-1 [&_h1]:text-lg [&_h1]:font-semibold [&_h2]:my-1 [&_h2]:text-[15px] [&_h2]:font-semibold [&_h3]:my-1 [&_h3]:font-semibold [&_ul]:my-1 [&_ul]:list-disc [&_ul]:pl-4 [&_ol]:my-1 [&_ol]:list-decimal [&_ol]:pl-4 [&_blockquote]:my-1 [&_blockquote]:border-l-2 [&_blockquote]:border-neutral-300 [&_blockquote]:pl-2 [&_blockquote]:text-neutral-500 [&_pre]:my-1 [&_pre]:rounded [&_pre]:bg-neutral-100 [&_pre]:p-1 [&_pre]:text-xs [&_code]:rounded [&_code]:bg-neutral-100 [&_code]:px-0.5 [&_a]:text-blue-600 [&_a]:underline';

/** remove слэш-текста перед применением команды. */
function commitKind(editor: Editor, kind: SlashCommandKind, queryText: string): boolean {
  const { state } = editor;
  const { $from } = state.selection;
  const start = Math.max(0, $from.pos - queryText.length);
  const base = editor.chain().focus().setTextSelection({ from: start, to: $from.pos }).deleteSelection();
  switch (kind) {
    case 'h1':
      return base.toggleHeading({ level: 1 }).run();
    case 'h2':
      return base.toggleHeading({ level: 2 }).run();
    case 'h3':
      return base.toggleHeading({ level: 3 }).run();
    case 'bulletList':
      return base.toggleBulletList().run();
    case 'orderedList':
      return base.toggleOrderedList().run();
    case 'blockquote':
      return base.toggleBlockquote().run();
    case 'codeBlock':
      return base.toggleCodeBlock().run();
    case 'hr':
      return base.setHorizontalRule().run();
    case 'link':
    case 'image':
      // Текст '/' убираем, дальнейшее — через UI (URL-инпут / библиотеку ассетов).
      base.run();
      return true;
  }
}

export function RichTextEditor({
  value,
  onChange,
  siteId,
  editorRef,
}: {
  value: string;
  onChange: (html: string) => void;
  siteId: string;
  editorRef?: (editor: Editor | null) => void;
}) {
  const { t } = useAdmin();
  const onChangeRef = useRef(onChange);
  onChangeRef.current = onChange;
  const editorRefLocal = useRef<Editor | null>(null);

  const [focused, setFocused] = useState(false);
  const [slash, setSlash] = useState<{ query: string } | null>(null);
  const [active, setActive] = useState(0);
  const [linkMode, setLinkMode] = useState(false);
  const [linkHref, setLinkHref] = useState('');
  const [pickerOpen, setPickerOpen] = useState(false);

  const keymapRef = useRef<(key: string) => boolean>(() => false);
  const slashKeymap = useMemo(() => createSlashKeymap(keymapRef), []);

  const editor = useEditor(
    {
      extensions: [
        slashKeymap,
        StarterKit.configure({ heading: { levels: [1, 2, 3] } }),
        Link.configure({ openOnClick: false, autolink: true }),
        Image.configure({ inline: false, allowBase64: true }),
      ],
      content: normalizeRichHtml(value),
      onUpdate: ({ editor }) => onChangeRef.current(editor.getHTML()),
    },
    [],
  );

  useEffect(() => {
    editorRefLocal.current = editor;
    lastEditor = editor;
    editorRef?.(editor);
    return () => {
      editorRefLocal.current = null;
      if (lastEditor === editor) lastEditor = null;
      editorRef?.(null);
    };
  }, [editor, editorRef]);

  // Внешнее значение → редактор, но не во время активного набора.
  useEffect(() => {
    if (!editor) return;
    if (!focused && editor.getHTML() !== normalizeRichHtml(value)) {
      editor.commands.setContent(normalizeRichHtml(value));
    }
  }, [editor, value, focused]);

  const visibleItems = useMemo(() => {
    if (!slash) return [];
    return matchSlashCommands(slash.query, (kind) => t(labelKeyFor(kind)));
  }, [slash, t]);

  useEffect(() => {
    if (!editor) return;
    const updateSlash = () => {
      if (linkMode || pickerOpen) return;
      const text = getTextBeforeCursor(editor);
      if (text.startsWith('/')) {
        setSlash((cur) => {
          const next = text.slice(1);
          if (cur?.query === next) return cur;
          setActive(0);
          return { query: next };
        });
      } else {
        setSlash(null);
      }
    };
    editor.on('update', updateSlash);
    editor.on('selectionUpdate', updateSlash);
    return () => {
      editor.off('update', updateSlash);
      editor.off('selectionUpdate', updateSlash);
    };
  }, [editor, linkMode, pickerOpen]);

  useEffect(() => {
    if (!slash || linkMode || pickerOpen) return;
    if (visibleItems.length === 0) return;
    keymapRef.current = (key) => {
      if (key === 'ArrowDown') {
        setActive((a) => (a + 1) % visibleItems.length);
        return true;
      }
      if (key === 'ArrowUp') {
        setActive((a) => (a - 1 + visibleItems.length) % visibleItems.length);
        return true;
      }
      if (key === 'Escape') {
        setSlash(null);
        return true;
      }
      if (key === 'Enter') {
        const kind = visibleItems[active];
        if (kind) {
          trigger(kind);
          return true;
        }
      }
      return false;
    };
    return () => {
      keymapRef.current = () => false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [slash, linkMode, pickerOpen, visibleItems, active, editor]);

  const closeOverlays = () => {
    setSlash(null);
    setLinkMode(false);
    setLinkHref('');
    setPickerOpen(false);
  };

  const trigger = (kind: ToolbarKind) => {
    if (!editor) return;
    if (kind === 'link') {
      if (slash) {
        const { state } = editor;
        const start = Math.max(0, state.selection.$from.pos - getTextBeforeCursor(editor).length);
        editor.chain().setTextSelection({ from: start, to: state.selection.$from.pos }).deleteSelection().run();
      }
      setSlash(null);
      setLinkHref('');
      setLinkMode(true);
      return;
    }
    if (kind === 'bold' || kind === 'italic' || kind === 'strike' || kind === 'code') {
      // Только из тулбара (в slash-меню базовых команд нет).
      if (!slash) {
        runInlineMark(editor, kind);
      }
      return;
    }
    if (kind === 'image') {
      if (slash) commitKind(editor, 'image', `/${slash.query}`);
      setSlash(null);
      setPickerOpen(true);
      return;
    }
    if (slash) {
      commitKind(editor, kind, `/${slash.query}`);
    } else {
      runSlashCommand(editor, kind);
    }
    setSlash(null);
    setActive(0);
  };

  const applyLink = () => {
    if (!editor || !linkHref.trim()) return;
    editor.chain().focus().extendMarkRange('link').setLink({ href: linkHref.trim() }).run();
    closeOverlays();
  };

  const applyImage = (assetId: string) => {
    if (!editor) return;
    const src = /^[\w-]+:/.test(assetId) || assetId.startsWith('/') ? assetId : `/api/assets/${assetId}/file`;
    editor.chain().focus().setImage({ src }).run();
    closeOverlays();
  };

  const toolbar = TOOLBAR_ORDER;

  return (
    <div className="relative rounded border border-neutral-200">
      <div className="flex flex-wrap items-center gap-1 border-b border-neutral-200 bg-neutral-50 px-1 py-1">
        {toolbar.map((kind) => (
          <button
            key={kind}
            type="button"
            className={TOOL_CLASS}
            aria-label={t(labelKeyFor(kind))}
            title={t(labelKeyFor(kind))}
            onClick={() => trigger(kind)}
          >
            {kind === 'h1' ? 'H1' : kind === 'h2' ? 'H2' : kind === 'h3' ? 'H3' : TOOL_GLYPH[kind]}
          </button>
        ))}
      </div>
      <div className={EDITOR_CLASS} data-testid="rich-text-body" data-rich-text="">
        <EditorContent
          editor={editor}
          onFocus={() => setFocused(true)}
          onBlur={() => setFocused(false)}
        />
      </div>
      <span className="pointer-events-none absolute bottom-1 right-1.5 text-[10px] text-neutral-300">
        {t('richText.slash.hint')}
      </span>

      {slash && visibleItems.length > 0 && (
        <div
          className="absolute left-2 top-12 z-20 min-w-44 rounded border border-neutral-200 bg-white py-1 shadow-md"
          role="listbox"
          aria-label={t('richText.slash.hint')}
        >
          {visibleItems.map((kind, i) => (
            <button
              key={kind}
              type="button"
              role="option"
              aria-selected={i === active}
              className={`flex w-full items-center gap-2 px-2 py-1 text-left text-xs ${
                i === active ? 'bg-blue-50 text-blue-700' : 'text-neutral-700'
              }`}
              onMouseEnter={() => setActive(i)}
              onClick={() => trigger(kind)}
            >
              {t(labelKeyFor(kind))}
            </button>
          ))}
        </div>
      )}

      {linkMode && (
        <div
          className="absolute left-2 top-12 z-20 flex items-center gap-1 rounded border border-neutral-200 bg-white px-2 py-1 shadow-md"
          role="dialog"
          aria-label={t('richText.toolbar.link')}
          // Останавливаем всплытие в редактор, чтобы Enter не сработал как команда.
          onMouseDown={(e) => e.preventDefault()}
        >
          <input
            autoFocus
            value={linkHref}
            onChange={(e) => setLinkHref(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') applyLink();
              if (e.key === 'Escape') closeOverlays();
            }}
            placeholder={t('richText.link.placeholder')}
            className="w-40 rounded border border-neutral-300 px-1.5 py-0.5 text-xs focus:border-blue-500 focus:outline-none"
            aria-label={t('richText.link.placeholder')}
          />
          <button type="button" className={TOOL_CLASS} onClick={applyLink} aria-label={t('richText.link.apply')}>
            {t('richText.link.apply')}
          </button>
        </div>
      )}

      <AssetPicker
        open={pickerOpen}
        onClose={() => setPickerOpen(false)}
        siteId={siteId}
        onSelect={applyImage}
      />
    </div>
  );
}

const TOOL_GLYPH: Record<string, string> = {
  bold: 'B',
  italic: 'I',
  strike: 'S',
  code: '</>',
  bulletList: '•',
  orderedList: '1.',
  blockquote: '❝',
  codeBlock: '⌘',
  hr: '—',
  link: '🔗',
  image: '▣',
};