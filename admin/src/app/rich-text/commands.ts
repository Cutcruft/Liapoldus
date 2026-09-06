import type { Editor } from '@tiptap/core';
import type { AdminStringKey } from '../../runtime';

export type SlashCommandKind =
  | 'h1'
  | 'h2'
  | 'h3'
  | 'bulletList'
  | 'orderedList'
  | 'blockquote'
  | 'codeBlock'
  | 'hr'
  | 'link'
  | 'image';

/** Дополнительные inline-форматы тулбара (в slash-меню не входят). */
export type ToolbarKind = SlashCommandKind | 'bold' | 'italic' | 'strike' | 'code';

export const TOOLBAR_ORDER: ToolbarKind[] = [
  'bold',
  'italic',
  'strike',
  'code',
  'h1',
  'h2',
  'h3',
  'bulletList',
  'orderedList',
  'blockquote',
  'codeBlock',
  'hr',
  'link',
  'image',
];

export const SLASH_COMMANDS: SlashCommandKind[] = [
  'h1',
  'h2',
  'h3',
  'bulletList',
  'orderedList',
  'blockquote',
  'codeBlock',
  'hr',
  'link',
  'image',
];

const LABEL_KEY: Record<ToolbarKind, AdminStringKey> = {
  h1: 'richText.toolbar.h1',
  h2: 'richText.toolbar.h2',
  h3: 'richText.toolbar.h3',
  bulletList: 'richText.toolbar.bulletList',
  orderedList: 'richText.toolbar.orderedList',
  blockquote: 'richText.toolbar.blockquote',
  codeBlock: 'richText.toolbar.codeBlock',
  hr: 'richText.toolbar.hr',
  link: 'richText.toolbar.link',
  image: 'richText.toolbar.image',
  bold: 'richText.toolbar.bold',
  italic: 'richText.toolbar.italic',
  strike: 'richText.toolbar.strike',
  code: 'richText.toolbar.code',
};

export function labelKeyFor(kind: ToolbarKind): AdminStringKey {
  return LABEL_KEY[kind];
}

export function matchSlashCommands(query: string, labelOf: (kind: SlashCommandKind) => string): SlashCommandKind[] {
  const q = query.trim().toLowerCase();
  if (!q) return SLASH_COMMANDS;
  return SLASH_COMMANDS.filter((kind) => labelOf(kind).toLowerCase().includes(q));
}

/** Текст от начала текущего блока («абзаца») до курсора — для детекта `/команды`. */
export function getTextBeforeCursor(editor: Editor): string {
  const { state } = editor;
  const { $from } = state.selection;
  if ($from.depth === 0) return '';
  return $from.parent.textBetween(0, $from.parentOffset, '\n', '');
}

/** Применение команды к редактору. Для 'link'/'image' обработка на стороне UI. */
export function runSlashCommand(editor: Editor, kind: SlashCommandKind): boolean {
  switch (kind) {
    case 'h1':
      return editor.chain().focus().toggleHeading({ level: 1 }).run();
    case 'h2':
      return editor.chain().focus().toggleHeading({ level: 2 }).run();
    case 'h3':
      return editor.chain().focus().toggleHeading({ level: 3 }).run();
    case 'bulletList':
      return editor.chain().focus().toggleBulletList().run();
    case 'orderedList':
      return editor.chain().focus().toggleOrderedList().run();
    case 'blockquote':
      return editor.chain().focus().toggleBlockquote().run();
    case 'codeBlock':
      return editor.chain().focus().toggleCodeBlock().run();
    case 'hr':
      return editor.chain().focus().setHorizontalRule().run();
    case 'link':
    case 'image':
      return false;
  }
}

/** Inline-форматы тулбара. */
export function runInlineMark(editor: Editor, kind: 'bold' | 'italic' | 'strike' | 'code'): boolean {
  switch (kind) {
    case 'bold':
      return editor.chain().focus().toggleBold().run();
    case 'italic':
      return editor.chain().focus().toggleItalic().run();
    case 'strike':
      return editor.chain().focus().toggleStrike().run();
    case 'code':
      return editor.chain().focus().toggleCode().run();
    default:
      return false;
  }
}