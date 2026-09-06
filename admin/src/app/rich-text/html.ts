/** Утилиты HTML round-trip для богатого текста (Tiptap). */

export function isEmptyHtml(html: string): boolean {
  const value = (html ?? '').trim();
  if (!value) return true;
  if (/^<p>(<br\s*\/?>|&nbsp;|\s)*<\/p>$/.test(value)) return true;
  return false;
}

export function htmlToText(html: string): string {
  const value = html ?? '';
  const doc = new DOMParser().parseFromString(value, 'text/html');
  return (doc.body.textContent ?? '').replace(/\s+/g, ' ').trim();
}

/** Нормализация: пустой редактор хранится как `<p></p>` (стабильный round-trip). */
export function normalizeRichHtml(html: string): string {
  const value = (html ?? '').trim();
  return isEmptyHtml(value) ? '<p></p>' : value;
}