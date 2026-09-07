/** Каталог компонентов редактора и их JSON-Schema (подмножество draft-07 для генератора форм). */

import { useEffect, useMemo, useState } from 'react';
import type { AdminApi } from '../../runtime/api';

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
  /**
   * section, который может быть каркасом страницы (принимает children
   * как лист контента, §1.3). Кандидат в layout-пикер страницы/сайта.
   */
  acceptsPageContent?: boolean;
}

/** Backend отдаёт каталог как `{type,label,container,schema}`. */
type RawCatalogEntry = {
  type: string;
  label: string;
  container: boolean;
  acceptsPageContent?: boolean;
  schema: unknown;
};

/** Каталог, отправленный с сервера, в схему редактора. Невалидные записи пропускаются. */
export function parseCatalog(raw: unknown): BuiltinComponent[] {
  if (!Array.isArray(raw)) return [];
  const out: BuiltinComponent[] = [];
  for (const entry of raw) {
    const e = entry as RawCatalogEntry;
    if (typeof e?.type !== 'string' || e.type === '') continue;
    if (typeof e?.label !== 'string' || e.label === '') continue;
    const schema = e.schema as JSONSchema | null;
    if (typeof schema !== 'object' || schema === null) continue;
    if (schema.type !== 'object') continue;
    if (typeof schema.properties !== 'object' || schema.properties === null) continue;
    out.push({ type: e.type, label: e.label, container: e.container === true, acceptsPageContent: e.acceptsPageContent === true, schema });
  }
  return out;
}

/** Синхронный fallback: используется, пока каталог не загружен с бэкенда. */
const FALLBACK_BUILTINS: BuiltinComponent[] = [
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

/** Статический индекс fallback-каталога (используется тестами и до загрузки). */
export const BUILTIN_BY_TYPE: Record<string, BuiltinComponent> = Object.fromEntries(
  FALLBACK_BUILTINS.map((c) => [c.type, c]),
);

/** Загрузка каталога с бэкенда; при ошибке/невалидном ответе — fallback. */
export async function loadCatalog(
  api: AdminApi,
  siteId: string,
): Promise<BuiltinComponent[]> {
  try {
    const res = await api.request('GET', `/api/sites/${encodeURIComponent(siteId)}/components`);
    if (!res.ok) return FALLBACK_BUILTINS;
    const parsed = parseCatalog(res.body);
    return parsed.length > 0 ? parsed : FALLBACK_BUILTINS;
  } catch {
    return FALLBACK_BUILTINS;
  }
}

export interface ComponentCatalog {
  components: BuiltinComponent[];
  byType: Record<string, BuiltinComponent>;
  schemasByType: Record<string, JSONSchema>;
}

/**
 * Каталог компонентов редактора. Изначально синхронный fallback (кнопки
 * палитры работают сразу), а после успешной загрузки страницы заменяется
 * полным каталогом с бэкенда (builtin + определённые на сайте компоненты).
 * `enabled` должен становиться true только когда редактор готов к работе.
 */
export function useComponentCatalog(
  api: AdminApi,
  siteId: string,
  enabled: boolean,
): ComponentCatalog {
  const [components, setComponents] = useState<BuiltinComponent[]>(FALLBACK_BUILTINS);

  useEffect(() => {
    if (!enabled || siteId === '') return;
    let alive = true;
    void loadCatalog(api, siteId).then((next) => {
      if (alive) setComponents(next);
    });
    return () => {
      alive = false;
    };
  }, [api, siteId, enabled]);

  const byType = useMemo(
    () => Object.fromEntries(components.map((c) => [c.type, c])),
    [components],
  );
  const schemasByType = useMemo(
    () => Object.fromEntries(components.map((c) => [c.type, c.schema])),
    [components],
  );

  return { components, byType, schemasByType };
}