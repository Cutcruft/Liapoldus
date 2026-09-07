import { render } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { ComponentRegistry } from '../../src/core/component-registry';
import { componentMapFromRegistry, registerBuiltinComponents } from '../../src/react/builtin';
import { PageRenderer } from '../../src/react/render';
import type { ResolvedElementNode } from '../../src/types/page';

function renderRoot(definitionId: string, props: Record<string, unknown>) {
  const elements: ResolvedElementNode[] = [
    { id: 'only', componentId: definitionId, props, bindings: [] },
  ];
  return render(<PageRenderer components={componentMapFromRegistry()} elements={elements} />);
}

describe('builtin Container', () => {
  it('рендерит div с layout stack (column)', () => {
    const { container } = renderRoot('Container', { layout: 'stack', gap: 8 });
    const div = container.querySelector('div[data-component="Container"]') as HTMLElement;
    expect(div).not.toBeNull();
    expect(div.style.display).toBe('flex');
    expect(div.style.flexDirection).toBe('column');
    expect(div.style.gap).toBe('8px');
  });

  it('layout grid переключает на grid', () => {
    const { container } = renderRoot('Container', { layout: 'grid', gap: 4 });
    const div = container.querySelector('div[data-component="Container"]') as HTMLElement;
    expect(div.style.display).toBe('grid');
    expect(div.style.gridTemplateColumns).toContain('repeat(auto-fit');
  });
});

describe('builtin Text', () => {
  it('рендерит html из props.text (richtext)', () => {
    const { container } = renderRoot('Text', { text: '<strong>Важно</strong>' });
    const span = container.querySelector('span[data-component="Text"]') as HTMLElement;
    expect(span).not.toBeNull();
    expect(span.innerHTML).toContain('<strong>Важно</strong>');
  });

  it('применяет size/align/color', () => {
    const { container } = renderRoot('Text', { text: 'x', size: 'xl', align: 'center', color: '#f00' });
    const span = container.querySelector('span[data-component="Text"]') as HTMLElement;
    expect(span.style.fontSize).toBe('30px');
    expect(span.style.textAlign).toBe('center');
    expect(span.style.color).toBe('rgb(255, 0, 0)');
  });
});

describe('builtin Image', () => {
  it('прямой URL используется как есть', () => {
    const { container } = renderRoot('Image', { assetId: 'https://x/y.png', alt: 'a' });
    const img = container.querySelector('img[data-component="Image"]') as HTMLImageElement;
    expect(img.src).toBe('https://x/y.png');
    expect(img.alt).toBe('a');
  });

  it('без RuntimeProvider фоллбэк на /api/assets/{id}/file', () => {
    const { container } = renderRoot('Image', { assetId: 'abc123', width: 120 });
    const img = container.querySelector('img[data-component="Image"]') as HTMLImageElement;
    expect(img.src).toContain('/api/assets/abc123/file');
    expect(img.style.width).toBe('120px');
  });
});

describe('builtin Button', () => {
  it('рендерит label как <button type=button>', () => {
    const { container } = renderRoot('Button', { label: 'Нажми' });
    const btn = container.querySelector('button[data-component="Button"]') as HTMLButtonElement;
    expect(btn.textContent).toBe('Нажми');
    expect(btn.type).toBe('button');
  });
});

describe('registerBuiltinComponents && componentMapFromRegistry', () => {
  beforeEach(() => {
    registerBuiltinComponents();
  });

  afterEach(() => {
    ComponentRegistry.clear();
  });

  it('регистрирует 4 builtin-компонента в ComponentRegistry', () => {
    expect(ComponentRegistry.hasDefinition('Container')).toBe(true);
    expect(ComponentRegistry.hasDefinition('Text')).toBe(true);
    expect(ComponentRegistry.hasDefinition('Image')).toBe(true);
    expect(ComponentRegistry.hasDefinition('Button')).toBe(true);
  });

  it('идемпотентен (повторная регистрация не бросает)', () => {
    expect(() => registerBuiltinComponents()).not.toThrow();
  });

  it('componentMapFromRegistry включает builtin и пользовательские определения', () => {
    const UserComp = () => null;
    ComponentRegistry.registerDefinition('hero', UserComp);
    const map = componentMapFromRegistry();
    expect(map['Container']).toBeDefined();
    expect(map['hero']).toBe(UserComp);
  });
});