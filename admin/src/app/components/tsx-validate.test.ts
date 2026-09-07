import { describe, expect, it } from 'vitest';
import { validateTsx } from './tsx-validate';

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