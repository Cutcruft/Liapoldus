import { describe, expect, it } from 'vitest';
import {
  hasDefaultExport,
  normalizeAllowlist,
  siteComponentImports,
  validateImportPolicy,
  validateTsx,
} from './tsx-validate';

describe('siteComponentImports (R10)', () => {
  it('находит статические и динамические импорты @site/components', () => {
    const src = [
      'import Card from "@site/components/card";',
      'import Header from \'@site/components/header\';',
      'const F = React.lazy(() => import("@site/components/footer"));',
      'import { Box } from "./local";',
    ].join('\n');
    expect(siteComponentImports(src)).toEqual(['card', 'header', 'footer']);
  });

  it('не цепляет относительные и сторонние импорты', () => {
    expect(siteComponentImports('import { x } from "./comp"; import z from "react";')).toEqual([]);
  });
});

describe('hasDefaultExport / normalizeAllowlist (R10)', () => {
  it('распознаёт короткую форму default-экспорта', () => {
    expect(hasDefaultExport('export default function A() {}')).toBe(true);
    expect(hasDefaultExport('export default () => null')).toBe(true);
    expect(hasDefaultExport('export function a() {}')).toBe(false);
  });

  it('нормализует allowlist без дублей и в отсортированном виде', () => {
    expect(normalizeAllowlist(['b', '', 'a', 'b'])).toEqual(['a', 'b']);
    expect(normalizeAllowlist()).toEqual([]);
  });
});

describe('validateImportPolicy (R10)', () => {
  const registry = [
    { id: 'text', isSection: false },
    { id: 'button', isSection: false },
    { id: 'main', isSection: true },
  ];

  it('примитивы корректны без импортов и без внутренних флагов', () => {
    expect(validateImportPolicy('export default () => null', registry, false, false, undefined)).toEqual([]);
  });

  it('примитив с импортом сайт-компонента отклоняется', () => {
    const errs = validateImportPolicy(
      'import T from "@site/components/text"; export default () => null',
      registry,
      false,
      false,
      undefined,
    );
    expect(errs.some((e) => e.includes('Примитив не может импортировать'))).toBe(true);
  });

  it('acceptsPageContent без isSection отклоняется', () => {
    const errs = validateImportPolicy('export default () => null', registry, false, true, undefined);
    expect(errs.some((e) => e.includes('acceptsPageContent требует isSection'))).toBe(true);
  });

  it('отсутствующий default-экспорт отклоняется', () => {
    const errs = validateImportPolicy('function A() { return null }', registry, false, false, undefined);
    expect(errs.some((e) => e.includes('default-экспорт'))).toBe(true);
  });

  it('секция с импортом из allowlist валидна', () => {
    const errs = validateImportPolicy(
      'import T from "@site/components/text"; export default () => null',
      registry,
      true,
      false,
      ['text'],
    );
    expect(errs).toEqual([]);
  });

  it('секция с импортом вне allowlist отклоняется', () => {
    const errs = validateImportPolicy(
      'import B from "@site/components/button"; export default () => null',
      registry,
      true,
      false,
      ['text'],
    );
    expect(errs.some((e) => e.includes('не включён в allowlist'))).toBe(true);
  });

  it('allowlist на неизвестную или секцию-цель отклоняется', () => {
    const unknown = validateImportPolicy('export default () => null', registry, true, false, ['ghost']);
    expect(unknown.some((e) => e.includes('неизвестный компонент ghost'))).toBe(true);
    const section = validateImportPolicy('export default () => null', registry, true, false, ['main']);
    expect(section.some((e) => e.includes('Allowlist ссылается на секцию main'))).toBe(true);
  });

  it('секция не может импортировать секцию', () => {
    const errs = validateImportPolicy(
      'import M from "@site/components/main"; export default () => null',
      registry,
      true,
      false,
      ['main'],
    );
    expect(errs.some((e) => e.includes('не может импортировать секцию'))).toBe(true);
  });
});

describe('validateTsx (sucrase, R5)', () => {
  it('пустой/пробельный исходник считается валидным', () => {
    expect(validateTsx('').ok).toBe(true);
    expect(validateTsx('   \n ').ok).toBe(true);
  });

  it('валидный TSX-компонент проходит', () => {
    const src = [
      'interface Props { title: string }',
      'export default function Card({ title }: Props) {',
      '  return <section><h1>{title}</h1></section>',
      '}',
    ].join('\n');
    expect(validateTsx(src)).toEqual({ ok: true });
  });

  it('незакрытый JSX возвращает строку ошибки', () => {
    const res = validateTsx('export default function A() { return <div> }');
    expect(res.ok).toBe(false);
    expect(res.line).toBe(1);
    expect(res.message).toBeTruthy();
  });

  it('не закрытая скобка ловится с позицией', () => {
    const res = validateTsx('export default function A() {\n  const s = "x"\n  return s\n');
    expect(res.ok).toBe(false);
    expect(res.line).toBeDefined();
  });

  it('не соответствие типов НЕ ловится (транспиляция без проверки типов)', () => {
    expect(validateTsx('const n: number = "text";').ok).toBe(true);
  });
});