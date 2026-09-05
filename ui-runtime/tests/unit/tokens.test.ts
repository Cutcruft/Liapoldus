import { describe, expect, it } from 'vitest';
import { DescriptorValidationError } from '../../src/errors';
import { DesignTokens } from '../../src/core/tokens';

describe('DesignTokens (§13)', () => {
  it('apply(theme) пишет CSS-переменные в target scope (:root)', () => {
    const css: string[] = [];
    const dt = new DesignTokens({ env: { writeCss: (c) => css.push(c) } });
    dt.apply({
      themeId: 'default',
      tokens: { '--color-primary': '#305EA8' },
    });
    expect(css.join('\n')).toContain(':root {');
  });

  it('токены записываются как --name: value', () => {
    const css: string[] = [];
    const dt = new DesignTokens({ env: { writeCss: (c) => css.push(c) } });
    dt.apply({
      themeId: 'default',
      tokens: { '--color-primary': '#305EA8' },
    });
    expect(css[0]).toContain('--color-primary: #305EA8;');
  });

  it('apply дважды (разные темы) → полное замещение', () => {
    const css: string[] = [];
    const dt = new DesignTokens({ env: { writeCss: (c) => css.push(c) } });
    dt.apply({
      themeId: 'default',
      tokens: { '--color-primary': '#305EA8', '--color-bg': '#FFFFFF' },
    });
    dt.apply({
      themeId: 'dark',
      tokens: { '--color-primary': '#7EA8E8' },
    });
    expect(css[1]).toContain('--color-primary: #7EA8E8;');
    expect(css[1]).not.toContain('--color-bg');
    expect(dt.get('--color-primary')).toBe('#7EA8E8');
    expect(dt.get('--color-bg')).toBeUndefined();
  });

  it('get(name) возвращает значение; неизвестный → undefined', () => {
    const dt = new DesignTokens();
    dt.apply({ themeId: 'default', tokens: { '--color-primary': '#305EA8' } });
    expect(dt.get('--color-primary')).toBe('#305EA8');
    expect(dt.get('--color-nope')).toBeUndefined();
  });

  it('тема с fonts подключает шрифты через link в управляемой DOM-среде', () => {
    const links: Array<{ rel?: string; type?: string; href?: string }> = [];
    const fonts: string[][] = [];
    const dt = new DesignTokens({
      env: {
        onFonts: (f) => fonts.push(f),
        document: {
          createElement: () => {
            const link = { rel: '', type: '', href: '' };
            links.push(link);
            return link;
          },
          head: { appendChild: () => undefined },
        },
      },
    });
    dt.apply({ themeId: 'dark', tokens: { '--font': 'Inter' }, fonts: ['Inter'] });
    expect(fonts[0]).toEqual(['Inter']);
    expect(links).toHaveLength(1);
    expect(links[0].rel).toBe('stylesheet');
    expect(links[0].href).toBe('Inter');
  });

  it('токен с ref резолвится перед применением', () => {
    const css: string[] = [];
    const dt = new DesignTokens({ env: { writeCss: (c) => css.push(c) } });
    dt.apply({
      themeId: 'default',
      tokens: {
        '--base': '#123456',
        '--color-primary': { ref: '--base' },
      },
    });
    expect(css[0]).toContain('--color-primary: #123456;');
  });

  it('невалидный токен → DescriptorValidationError', () => {
    const dt = new DesignTokens();
    expect(() =>
      dt.apply({ themeId: 'x', tokens: { '--bg': { value: 42 } as never } }),
    ).toThrow(DescriptorValidationError);
    expect(() =>
      dt.apply({ themeId: 'y', tokens: { '--a': { ref: '--missing' } } }),
    ).toThrow(DescriptorValidationError);
  });
});