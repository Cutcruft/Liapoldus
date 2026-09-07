import { describe, expect, it } from 'vitest';
import { inferProps, inferComponentName } from './schema-utils';

const SNIPPET = [
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
].join('\n');

describe('inferProps (эвристика R5)', () => {
  it('выводит props из interface Props', () => {
    const props = inferProps(SNIPPET);
    expect(props).toEqual([{ name: 'title', type: 'string', required: true }]);
  });

  it('поддерживает type Props = { ... } (с равенством)', () => {
    const src = [
      'type Props = {',
      '  title: string;',
      '  count?: number;',
      '}',
      'export default function A({ title, count }: Props) { return null }',
    ].join('\n');
    expect(inferProps(src)).toEqual([
      { name: 'title', type: 'string', required: true },
      { name: 'count', type: 'number', required: false },
    ]);
  });

  it('необязательные и дефолты из деструктуризации параметра', () => {
    const src = [
      'interface Props { title: string; count?: number }',
      'export default function B({ title, count = 0 }: Props) { return null }',
    ].join('\n');
    const byName = new Map(inferProps(src).map((p) => [p.name, p]));
    expect(byName.get('title')).toEqual({ name: 'title', type: 'string', required: true });
    expect(byName.get('count')).toEqual({ name: 'count', type: 'number', required: false, default: '0' });
  });

  it('выводит инлайн-тип из (props: { ... })', () => {
    const src = 'const el = (props: { title: string }) => props.title;';
    expect(inferProps(src)).toEqual([{ name: 'title', type: 'string', required: true }]);
  });

  it('порядок — по появлению в исходнике, дубликат не плодится', () => {
    const src = [
      'interface Props { a: string; b: number }',
      'export default function C({ b }: Props) { return null }',
    ].join('\n');
    expect(inferProps(src).map((p) => p.name)).toEqual(['a', 'b']);
  });
});

describe('inferComponentName', () => {
  it('именованный default function', () => {
    expect(inferComponentName('export default function Card() { return null }')).toBe('Card');
  });

  it('именованный default const', () => {
    expect(inferComponentName('export default Card = () => <div/>')).toBe('Card');
  });

  it('анонимный default — имя из Props или "Component"', () => {
    expect(
      inferComponentName('export interface CardProps {}\nexport default ({}) => null'),
    ).toBe('CardProps');
    expect(inferComponentName('export default () => null')).toBe('Component');
  });

  it('без default-экспорта — null', () => {
    expect(inferComponentName('export const helper = () => 1')).toBeNull();
  });
});