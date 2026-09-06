import { describe, expect, it } from 'vitest';
import { render } from '@testing-library/react';
import { Box, Columns, Divider, Frame, Grid, Inline, Sidebar, Spacer, SplitPane, Stack } from '../index';

function assertClasses(el: Element | null, expected: string[]): void {
  expect(el).toBeTruthy();
  const actual = el!.className;
  for (const c of expected) {
    expect(actual, `ожидался класс "${c}" в "${actual}"`).toContain(c);
  }
}

function classesOf(container: HTMLElement): string {
  return (container.firstElementChild as Element).className;
}

describe('Box', () => {
  it('рендерит div с pad-классами', () => {
    const { container } = render(<Box pad={4}>x</Box>);
    assertClasses(container.firstElementChild, ['p-4']);
  });

  it('pad y → py-*, x → px-*', () => {
    const { container } = render(<Box pad={{ x: 2, y: 6 }} />);
    assertClasses(container.firstElementChild, ['px-2', 'py-6']);
  });

  it('grow → grow, as → тег', () => {
    const { container } = render(<Box as="section" grow />);
    expect(container.firstElementChild?.tagName).toBe('SECTION');
    assertClasses(container.firstElementChild, ['grow']);
  });

  it('передаёт className', () => {
    const { container } = render(<Box className="bg-red-50" />);
    assertClasses(container.firstElementChild, ['bg-red-50']);
  });
});

describe('Stack / Inline', () => {
  it.each([
    [{ gap: 4 }, 'flex flex-col gap-4'],
    [{ gap: 2, align: 'center' }, 'flex flex-col gap-2 items-center'],
    [{ gap: 1, justify: 'between', pad: 3 }, 'flex flex-col gap-1 justify-between p-3'],
    [{ align: 'stretch' }, 'flex flex-col items-stretch'],
    [{ gap: 6, align: 'end', justify: 'around' }, 'flex flex-col gap-6 items-end justify-around'],
  ])('Stack props %o → классы "%s"', (props, expected) => {
    const { container } = render(<Stack {...props} />);
    assertClasses(container.firstElementChild, expected.split(' '));
  });

  it.each([
    [{ gap: 2, wrap: true }, 'flex flex-row gap-2 flex-wrap'],
    [{ justify: 'start' }, 'flex flex-row justify-start'],
    [{ gap: 8 }, 'flex flex-row gap-8'],
  ])('Inline props %o → классы "%s"', (props, expected) => {
    const { container } = render(<Inline {...props} />);
    assertClasses(container.firstElementChild, expected.split(' '));
  });
});

describe('Columns / Grid', () => {
  it('count=3 → grid-cols-3', () => {
    const { container } = render(<Columns count={3} gap={4} />);
    assertClasses(container.firstElementChild, ['grid', 'grid-cols-3', 'gap-4']);
  });

  it('count вне диапазона клампится', () => {
    const { container } = render(<Columns count={99} />);
    assertClasses(container.firstElementChild, ['grid-cols-6']);
  });

  it('Grid cols/rows', () => {
    const { container } = render(<Grid cols={12} rows={2} gap={2} />);
    assertClasses(container.firstElementChild, ['grid', 'grid-cols-12', 'grid-rows-2', 'gap-2']);
  });
});

describe('Spacer / Divider', () => {
  it('Spacer → flex-1, aria-hidden', () => {
    const { container } = render(<Spacer />);
    assertClasses(container.firstElementChild, ['flex-1']);
    expect(container.firstElementChild!.getAttribute('aria-hidden')).toBe('true');
  });

  it('Divider horizontal → role separator', () => {
    const { container } = render(<Divider />);
    expect(container.firstElementChild!.getAttribute('role')).toBe('separator');
    expect(container.firstElementChild!.getAttribute('aria-orientation')).toBe('horizontal');
  });

  it('Divider vertical → self-stretch w-px mx-*', () => {
    const { container } = render(<Divider orientation="vertical" space={4} />);
    assertClasses(container.firstElementChild, ['self-stretch', 'w-px', 'mx-4']);
    expect(container.firstElementChild!.getAttribute('aria-orientation')).toBe('vertical');
  });
});

describe('Frame', () => {
  it('h-full overflow-auto по умолчанию', () => {
    const { container } = render(<Frame>c</Frame>);
    assertClasses(container.firstElementChild, ['h-full', 'overflow-auto']);
  });

  it('числовая высота → inline style без класс-генерации', () => {
    const { container } = render(<Frame height={400} />);
    const el = container.firstElementChild as HTMLElement;
    expect(el.style.height).toBe('400px');
    expect(classesOf(container)).not.toContain('h-[400px]');
  });

  it('overflow вариант', () => {
    const { container } = render(<Frame overflow="hidden" />);
    assertClasses(container.firstElementChild, ['overflow-hidden']);
  });
});

describe('Sidebar', () => {
  it('левая панель: панель + контент', () => {
    const { container } = render(
      <Sidebar width="sm">
        <div data-k="bar" />
        <div data-k="content" />
      </Sidebar>,
    );
    const bar = container.querySelector('[data-k="bar"]') as HTMLElement;
    const content = container.querySelector('[data-k="content"]') as HTMLElement;
    assertClasses(bar.parentElement, ['w-52', 'shrink-0']);
    assertClasses(content.parentElement, ['grow', 'overflow-auto']);
  });

  it('right → flex-row-reverse', () => {
    const { container } = render(
      <Sidebar side="right">
        <div />
        <div />
      </Sidebar>,
    );
    assertClasses(container.firstElementChild, ['flex-row-reverse']);
  });
});

describe('SplitPane', () => {
  it('horizontal: первая панель width по ratio (inline)', () => {
    const { container } = render(
      <SplitPane ratio={0.3}>
        <div data-k="a" />
        <div data-k="b" />
      </SplitPane>,
    );
    const a = container.querySelector('[data-k="a"]') as HTMLElement;
    expect(a.parentElement?.style.width).toBe('30%');
  });

  it('vertical: первая панель height + flex-col', () => {
    const { container } = render(
      <SplitPane direction="vertical" ratio={0.25} min={200}>
        <div data-k="a" />
        <div />
      </SplitPane>,
    );
    const a = container.querySelector('[data-k="a"]') as HTMLElement;
    assertClasses(container.firstElementChild, ['flex-col']);
    expect(a.parentElement?.style.height).toBe('25%');
    expect(a.parentElement?.style.minHeight).toBe('200px');
  });
});