/** Встроенные компоненты и их JSON-Schema (подмножество draft-07 для генератора форм). */

export type JSONSchemaValueType = 'string' | 'number' | 'boolean';

export type JSONSchemaProperty = {
  type: JSONSchemaValueType;
  title?: string;
  description?: string;
  default?: unknown;
  enum?: string[];
  min?: number;
  max?: number;
  required?: boolean;
  /** Семантика редактирования: 'asset' → asset-picker, 'richtext' → Tiptap. */
  format?: 'asset' | 'richtext';
};

export type JSONSchema = {
  type: 'object';
  title?: string;
  description?: string;
  default?: Record<string, unknown>;
  required?: string[];
  properties: Record<string, JSONSchemaProperty>;
};

export interface BuiltinComponent {
  type: string;
  label: string;
  schema: JSONSchema;
  /**
   * Разрешено ли вкладывать детей. false — листовой компонент
   * (появляется в меню «добавить» только для контейнерных родителя).
   */
  container: boolean;
}

export const BUILTIN_COMPONENTS: BuiltinComponent[] = [
  {
    type: 'Container',
    label: 'Контейнер',
    container: true,
    schema: {
      type: 'object',
      title: 'Контейнер',
      default: { layout: 'stack', gap: 8 },
      properties: {
        layout: {
          type: 'string',
          title: 'Раскладка',
          enum: ['stack', 'row', 'grid'],
          default: 'stack',
        },
        gap: { type: 'number', title: 'Отступ', default: 8, min: 0, max: 64 },
      },
    },
  },
  {
    type: 'Text',
    label: 'Текст',
    container: false,
    schema: {
      type: 'object',
      title: 'Текст',
      default: { text: 'Текст', size: 'md', align: 'left' },
      required: ['text'],
      properties: {
        text: { type: 'string', title: 'Текст', default: 'Текст', required: true, format: 'richtext' },
        size: {
          type: 'string',
          title: 'Размер',
          enum: ['sm', 'md', 'lg', 'xl'],
          default: 'md',
        },
        align: {
          type: 'string',
          title: 'Выравнивание',
          enum: ['left', 'center', 'right'],
          default: 'left',
        },
        color: { type: 'string', title: 'Цвет', default: '#111827' },
      },
    },
  },
  {
    type: 'Image',
    label: 'Картинка',
    container: false,
    schema: {
      type: 'object',
      title: 'Картинка',
      default: { alt: '', width: 320 },
      properties: {
        assetId: { type: 'string', title: 'Ассет', format: 'asset', default: '' },
        alt: { type: 'string', title: 'Alt', default: '' },
        width: { type: 'number', title: 'Ширина', default: 320, min: 1, max: 1920 },
      },
    },
  },
  {
    type: 'Button',
    label: 'Кнопка',
    container: false,
    schema: {
      type: 'object',
      title: 'Кнопка',
      default: { label: 'Кнопка', variant: 'primary' },
      required: ['label'],
      properties: {
        label: { type: 'string', title: 'Надпись', default: 'Кнопка', required: true },
        variant: {
          type: 'string',
          title: 'Вариант',
          enum: ['primary', 'secondary', 'ghost'],
          default: 'primary',
        },
      },
    },
  },
];

export const BUILTIN_BY_TYPE: Record<string, BuiltinComponent> = Object.fromEntries(
  BUILTIN_COMPONENTS.map((c) => [c.type, c]),
);

export function builtinFieldSchema(type: string): JSONSchema | undefined {
  return BUILTIN_BY_TYPE[type]?.schema;
}

export function isBuiltinType(type: string): boolean {
  return type in BUILTIN_BY_TYPE;
}