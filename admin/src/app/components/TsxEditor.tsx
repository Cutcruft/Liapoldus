import { useEffect, useMemo, useRef, useState } from 'react';
import { useEditor, EditorContent, type Editor } from '@tiptap/react';
import { Extension } from '@tiptap/core';
import StarterKit from '@tiptap/starter-kit';
import CodeBlockLowlight from '@tiptap/extension-code-block-lowlight';
import { createLowlight } from 'lowlight';
// Селективные грамматики highlight.js — не тащим все ~150 языков в бандл.
import typescript from 'highlight.js/lib/languages/typescript';
import javascript from 'highlight.js/lib/languages/javascript';
import xml from 'highlight.js/lib/languages/xml';
import json from 'highlight.js/lib/languages/json';
import bash from 'highlight.js/lib/languages/bash';
import { getTextBeforeCursor } from '../rich-text/commands';
import type { AdminStringKey } from '../../runtime';
import { useAdmin } from '../admin-context';
import { validateTsx, type TsxDiagnostic } from './tsx-validate';

const lowlight = createLowlight({ typescript, javascript, xml, json, bash });

/** Тестовый хук: последний смонтированный редактор TSX. */
let lastEditor: Editor | null = null;
export function __lastTsxEditor(): Editor | null {
  return lastEditor;
}

export type TsxSnippetKind = 'component' | 'section' | 'ref' | 'binding';

export const TSX_SNIPPETS: ReadonlyArray<{ kind: TsxSnippetKind; key: AdminStringKey; snippet: string }> = [
  {
    kind: 'component',
    key: 'components.slash.component',
    snippet: [
      'export interface Props {',
      '  title: string',
      '}',
      '',
      'export default function Component({ title }: Props) {',
      '  return (',
      '    <section>',
      '      <h1>{title}</h1>',
      '    </section>',
      '  )',
      '}',
    ].join('\n'),
  },
  {
    kind: 'section',
    key: 'components.slash.section',
    snippet: '<section>\n  {children}\n</section>',
  },
  {
    kind: 'ref',
    key: 'components.slash.ref',
    snippet: 'ref="name"',
  },
  {
    kind: 'binding',
    key: 'components.slash.binding',
    snippet: 'data-binding="path.to.field"',
  },
];

/** Кеймэп: стрелки/Enter/Escape отдают приоритет открытому slash-меню. */
function createSlashKeymap(keymapRef: { current: (key: string) => boolean }) {
  return Extension.create({
    name: 'tsxSlashKeymap',
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

/** Текст перед курсором открывает палитру только для «/» или «/слово» (без пробелов). */
function slashQuery(text: string): string | null {
  const seg = /[^\s\n]*$/.exec(text)?.[0] ?? '';
  if (seg === '/') return '';
  const m = /^\/([A-Za-z_$][\w]*)$/.exec(seg);
  if (m) return m[1] ?? null;
  return null;
}

const EDITOR_CLASS =
  'font-mono text-[13px] leading-relaxed focus:outline-none [&_pre]:m-0 [&_pre]:rounded [&_pre]:border [&_pre]:border-neutral-200 [&_pre]:bg-neutral-50 [&_pre]:p-3 [&_pre]:text-neutral-800';

export function TsxEditor({
  value,
  onChange,
  readOnly = false,
  onDiagnostic,
}: {
  value: string;
  onChange: (source: string) => void;
  readOnly?: boolean;
  onDiagnostic?: (d: TsxDiagnostic) => void;
}) {
  const { t } = useAdmin();
  const onChangeRef = useRef(onChange);
  onChangeRef.current = onChange;
  const onDiagRef = useRef(onDiagnostic);
  onDiagRef.current = onDiagnostic;

  /** Значение, которое редактор уже отражает (для внешней синхронизации и фильтрации
   *  отражённых/служебных update-событий, чтобы источник не помечался «не сохранён» на маунте). */
  const appliedRef = useRef(value);

  const [focused, setFocused] = useState(false);
  const [slash, setSlash] = useState<{ query: string } | null>(null);
  const [active, setActive] = useState(0);
  const [diagnostic, setDiagnostic] = useState<TsxDiagnostic>({ ok: true });

  const keymapRef = useRef<(key: string) => boolean>(() => false);
  const slashKeymap = useMemo(() => createSlashKeymap(keymapRef), []);

  const editor = useEditor(
    {
      extensions: [
        slashKeymap,
        StarterKit.configure({ codeBlock: false }),
        CodeBlockLowlight.configure({ lowlight }),
      ],
      editable: !readOnly,
      content: {
        type: 'doc',
        content: [
          {
            type: 'codeBlock',
            attrs: { language: 'typescript' },
            content: value ? [{ type: 'text', text: value }] : [],
          },
        ],
      },
      onUpdate: ({ editor: ed }) => {
        const text = ed.getText();
        // Начальный/отражённый update (текст не изменился относительно того, что мы
        // применили/прокинули последний раз) — не отдаём родителю, иначе источник
        // помечается «не сохранён» уже на маунте.
        if (text === appliedRef.current) return;
        appliedRef.current = text;
        onChangeRef.current(text);
      },
    },
    [],
  );

  useEffect(() => {
    lastEditor = editor;
    return () => {
      if (lastEditor === editor) lastEditor = null;
    };
  }, [editor]);

  useEffect(() => {
    editor?.setEditable(!readOnly);
  }, [editor, readOnly]);

  // Внешнее значение → редактор, но не во время активного набора.
  // Ответный onUpdate от setContent отсекается в onUpdate по равенству текста с appliedRef.
  useEffect(() => {
    if (!editor) return;
    if (value === appliedRef.current) return;
    if (readOnly || !focused) {
      appliedRef.current = value;
      editor.commands.setContent({
        type: 'doc',
        content: [
          {
            type: 'codeBlock',
            attrs: { language: 'typescript' },
            content: value ? [{ type: 'text', text: value }] : [],
          },
        ],
      });
    }
  }, [editor, value, focused, readOnly]);

  // Live-валидация (sucrase) с дебаунсом.
  useEffect(() => {
    const next = validateTsx(value);
    if (next.ok === diagnostic.ok && next.line === diagnostic.line && next.message === diagnostic.message) return;
    const timer = window.setTimeout(() => {
      const d = validateTsx(value);
      setDiagnostic(d);
      onDiagRef.current?.(d);
    }, 300);
    return () => window.clearTimeout(timer);
  }, [value]);

  // Slash-детект внутри code-блока.
  useEffect(() => {
    if (!editor) return;
    const updateSlash = () => {
      if (readOnly) return;
      const { $from } = editor.state.selection;
      if ($from.parent.type.name !== 'codeBlock') {
        setSlash(null);
        return;
      }
      const text = getTextBeforeCursor(editor);
      const query = slashQuery(text);
      if (query !== null) {
        setSlash((cur) => {
          if (cur?.query === query) return cur;
          setActive(0);
          return { query };
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
  }, [editor, readOnly]);

  const visible = TSX_SNIPPETS.filter((s) => {
    const q = slash?.query ?? '';
    if (!q) return true;
    return t(s.key).toLowerCase().includes(q.toLowerCase());
  });

  useEffect(() => {
    if (!slash) {
      keymapRef.current = () => false;
      return;
    }
    if (visible.length === 0) {
      keymapRef.current = () => false;
      setSlash(null);
      return;
    }
    const navigate = (delta: number) => (setActive((a) => (a + delta + visible.length) % visible.length), true);
    keymapRef.current = (key) => {
      if (key === 'ArrowDown') return navigate(1);
      if (key === 'ArrowUp') return navigate(-1);
      if (key === 'Escape') {
        setSlash(null);
        return true;
      }
      if (key === 'Enter') {
        const item = visible[active];
        if (item) {
          insertSnippet(editor, item.snippet);
          setSlash(null);
          setActive(0);
          return true;
        }
      }
      return false;
    };
    return () => {
      keymapRef.current = () => false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [slash, visible, active, editor]);

  function insertSnippet(ed: Editor | null, snippet: string) {
    if (!ed) return;
    const { $from } = ed.state.selection;
    const text = getTextBeforeCursor(ed);
    const from = Math.max(0, $from.pos - text.length);
    const to = $from.pos;
    // Вставляем как чистый текст (не HTML): иначе JSX-теги сниппета разбираются как элементы.
    ed.chain().focus().deleteRange({ from, to }).command(({ tr }) => {
      tr.insertText(snippet);
      return true;
    }).run();
  }

  return (
    <div className="relative rounded border border-neutral-200" data-testid="tsx-editor">
      <EditorContent
        editor={editor}
        className={EDITOR_CLASS}
        onFocus={() => setFocused(true)}
        onBlur={() => setFocused(false)}
      />
      <span className="pointer-events-none absolute bottom-1.5 right-2 text-[10px] text-neutral-300">
        {t('components.slash.hint')}
      </span>

      {slash && visible.length > 0 && (
        <div
          className="absolute left-2 top-2 z-20 min-w-52 rounded border border-neutral-200 bg-white py-1 shadow-md"
          role="listbox"
          aria-label={t('components.slash.hint')}
        >
          {visible.map((s, i) => (
            <button
              key={s.kind}
              type="button"
              role="option"
              aria-selected={i === active}
              className={`flex w-full items-center gap-2 px-2 py-1 text-left text-xs ${
                i === active ? 'bg-blue-50 text-blue-700' : 'text-neutral-700'
              }`}
              onMouseEnter={() => setActive(i)}
              onClick={() => {
                insertSnippet(editor, s.snippet);
                setSlash(null);
                setActive(0);
              }}
            >
              {t(s.key)}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}