/** Минимальный переводчик строк с подстановкой {var}. ru-тексты первичны. */

export type Translate = (key: string, vars?: Record<string, string | number>) => string;

export const DEFAULT_LANGUAGE = 'ru';

export function makeTranslate(entries: Record<string, string>, language = DEFAULT_LANGUAGE): Translate {
  const fallbackOf = (key: string): string => entries[key] ?? key;
  return (key, vars) => {
    let text = language === 'ru' ? fallbackOf(key) : (entries[`${language}.${key}`] ?? fallbackOf(key));
    if (vars) {
      for (const [k, v] of Object.entries(vars)) {
        text = text.split(`{${k}}`).join(String(v));
      }
    }
    return text;
  };
}