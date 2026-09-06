import { describe, expect, it } from 'vitest';
import { isEmptyHtml, htmlToText, normalizeRichHtml } from './html';

describe('html', () => {
  it('isEmptyHtml: пустые варианты', () => {
    expect(isEmptyHtml('')).toBe(true);
    expect(isEmptyHtml('   ')).toBe(true);
    expect(isEmptyHtml('<p></p>')).toBe(true);
    expect(isEmptyHtml('<p><br></p>')).toBe(true);
    expect(isEmptyHtml('<p> </p>')).toBe(true);
  });

  it('isEmptyHtml: непустой контент', () => {
    expect(isEmptyHtml('<p>Текст</p>')).toBe(false);
    expect(isEmptyHtml('<h1></h1>')).toBe(false);
    expect(isEmptyHtml('<p><strong>b</strong></p>')).toBe(false);
  });

  it('htmlToText: теги убираются, пробелы схлопываются', () => {
    expect(htmlToText('<p>Привет, <strong>мир</strong>!</p>')).toBe('Привет, мир!');
    expect(htmlToText('<h1>Заголовок</h1><ul><li>1</li></ul>')).toBe('Заголовок1');
    expect(htmlToText('')).toBe('');
  });

  it('normalizeRichHtml: пусто → стабильный <p></p> round-trip', () => {
    expect(normalizeRichHtml('')).toBe('<p></p>');
    expect(normalizeRichHtml('<p></p>')).toBe('<p></p>');
    expect(normalizeRichHtml('<p>Текст</p>')).toBe('<p>Текст</p>');
  });
});