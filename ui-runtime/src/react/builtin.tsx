import { createElement, type CSSProperties, type FC, type ReactNode } from 'react';
import { ComponentRegistry } from '../core/component-registry';
import type { ComponentMap } from './render';
import { useAsset } from './hooks';

/**
 * Builtin-компоненты рантайма (§11): Container/Text/Image/Button — реавшие
 * React-реализации встроенного каталога, который редактор отдаёт бэкендом
 * (backend/internal/application/component/builtin.go). page-дерево ссылается
 * на них по definitionId {Container,Text,Image,Button}; PageRenderer передаёт
 * props + резолвленные binding-значения. Metadata/schema здесь не зашиты.
 *
 * Проп-контракт повторяет design-mode (admin PreviewPane), чтобы собранная
 * страница выглядела как в редакторе.
 */

const TEXT_SIZE: Record<string, number> = { sm: 12, md: 16, lg: 22, xl: 30 };

/** Container: layout stack/row/grid + gap; контент — children из дерева. */
function Container(props: Record<string, unknown>) {
  const layout = String(props['layout'] ?? 'stack');
  const style: CSSProperties = {
    gap: Number(props['gap'] ?? 0),
    display: layout === 'grid' ? 'grid' : 'flex',
    flexDirection: layout === 'row' ? 'row' : layout === 'grid' ? undefined : 'column',
    gridTemplateColumns: layout === 'grid' ? 'repeat(auto-fit, minmax(0, 1fr))' : undefined,
  };
  return createElement('div', { 'data-component': 'Container', style }, props.children as ReactNode);
}

/** Text: richtext html из props.text; size/align/color. */
function Text(props: Record<string, unknown>) {
  const html = String(props['text'] ?? '');
  const style: CSSProperties = {
    fontSize: TEXT_SIZE[String(props['size'])] ?? 16,
    textAlign: (props['align'] as CSSProperties['textAlign']) ?? 'left',
    color: String(props['color'] ?? ''),
  };
  return createElement('span', {
    'data-component': 'Text',
    className: 'whitespace-pre-wrap',
    style,
    dangerouslySetInnerHTML: { __html: html },
  });
}

/** Image: assetId → URL из стора (useAsset), либо прямой URL/путь, либо
 *  fallback `/api/assets/{id}/file`. */
function Image(props: Record<string, unknown>) {
  const assetId = String(props['assetId'] ?? '');
  const resolved = useAsset({ assetId });
  const direct = /^[\w-]+:/.test(assetId) || assetId.startsWith('/');
  if (!assetId) {
    return createElement('div', { 'data-component': 'Image' });
  }
  const src = direct ? assetId : resolved ?? `/api/assets/${assetId}/file`;
  return createElement('img', {
    'data-component': 'Image',
    src,
    alt: String(props['alt'] ?? ''),
    style: { width: Number(props['width'] ?? 320) },
  });
}

/** Button: label из props.label. */
function Button(props: Record<string, unknown>) {
  return createElement(
    'button',
    { 'data-component': 'Button', type: 'button', className: 'rounded bg-blue-600 px-3 py-1.5 text-sm text-white' },
    String(props['label'] ?? ''),
  );
}

/** Иды builtin-компонентов (совпадают с node.type редактора). */
export const BUILTIN_COMPONENTS: Record<string, FC<Record<string, unknown>>> = {
  Container: Container,
  Text: Text,
  Image: Image,
  Button: Button,
};

/** Регистрирует builtin-компоненты в ComponentRegistry (идемпотентно). */
export function registerBuiltinComponents(): void {
  for (const [id, component] of Object.entries(BUILTIN_COMPONENTS)) {
    if (!ComponentRegistry.hasDefinition(id)) {
      ComponentRegistry.registerDefinition(id, component);
    }
  }
}

// Сообый эффект при загрузке модуля: builtin-компоненты доступны любой
// собранной странице без дополнительной инициализации.
registerBuiltinComponents();

/** Строит ComponentMap (definitionId → React-компонент) из ComponentRegistry
 *  на момент вызова: builtin + зарегистрированные определения site-бандла. */
export function componentMapFromRegistry(): ComponentMap {
  const map: ComponentMap = {};
  for (const id of ComponentRegistry.ids()) {
    map[id] = ComponentRegistry.getDefinition(id) as ComponentMap[string];
  }
  return map;
}