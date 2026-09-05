import { LocaleUnsupportedError } from '../errors';
import type { RuntimeStore } from './store';

export interface I18nOptions {
  defaultLocale: string;
  supportedLocales?: string[];
  /** резервная коллекция UI-строк (контент), default 'strings' */
  stringsCollection?: string;
  storageKey?: string;
  /** бросать LocaleUnsupportedError при неизвестной локали */
  strict?: boolean;
  /** колбэк переспроса локализованного контента при смене локали */
  syncContent?: (locale: string) => void | Promise<void>;
}

export interface I18nEnv {
  storage?: { getItem(key: string): string | null; setItem(key: string, value: string): void };
  navigatorLanguage?: string;
}

function normalize(locale: string): string {
  return locale.trim().split(/[-_]/)[0].toLowerCase();
}

/**
 * Локализация (спека §7a): localStorage → navigator.language → defaultLocale;
 * строки — контент коллекции `strings` в `store.content` (серверный фолбэк уже применён).
 */
export class I18n {
  readonly storageKey: string;
  private readonly listeners = new Set<(locale: string) => void>();

  constructor(
    private readonly opts: I18nOptions,
    private readonly store: RuntimeStore,
    private readonly env?: I18nEnv,
  ) {
    this.storageKey = opts.storageKey ?? 'liapoldus.locale';
  }

  /** localStorage → navigator.language → defaultLocale. */
  detect(): void {
    const stored = this.env?.storage?.getItem(this.storageKey) ?? null;
    const candidate = stored ?? this.env?.navigatorLanguage ?? this.opts.defaultLocale;
    const locale = this.resolveLocale(candidate);
    this.store.getState().setLocale(locale);
    if (stored !== locale) this.env?.storage?.setItem(this.storageKey, locale);
    this.notify(locale);
  }

  getLocale(): string {
    return this.store.getState().locale;
  }

  /** normalize + сохранить + уведомить + переспрос контента. */
  async setLocale(locale: string): Promise<void> {
    const resolved = this.resolveLocale(locale);
    const prev = this.store.getState().locale;
    this.store.getState().setLocale(resolved);
    this.env?.storage?.setItem(this.storageKey, resolved);
    if (prev !== resolved) this.notify(resolved);
    await this.opts.syncContent?.(resolved);
  }

  /** 'ru-RU' → 'ru'; при supportedLocales: совпавший → нормализованный, несовпавший → defaultLocale (strict — бросок). */
  resolveLocale(candidate: string): string {
    const normalized = normalize(candidate);
    const supported = this.opts.supportedLocales ?? [];
    if (supported.length === 0) return normalized;
    const found = supported.find((s) => normalize(s) === normalized);
    if (found) return normalize(found);
    if (this.opts.strict) throw new LocaleUnsupportedError(`Локаль '${candidate}' не поддерживается`);
    return this.opts.defaultLocale;
  }

  /** Подписка на смену локали; возвращаемая функция отписывает. */
  onLocaleChange(cb: (locale: string) => void): () => void {
    this.listeners.add(cb);
    return () => {
      this.listeners.delete(cb);
    };
  }

  /** Строка по dot-path в `store.content`; отсутствует → сырой ключ. */
  t(key: string, params?: Record<string, string | number>): string {
    const value = lookupByDot(this.store.getState().content, key);
    if (typeof value !== 'string') return key;
    return interpolate(value, params);
  }

  /** Плоский словарь всех строк текущего кантента (ключ — dot-path). */
  strings(): Record<string, string> {
    const out: Record<string, string> = {};
    for (const [id, entry] of Object.entries(this.store.getState().content)) {
      flatten(id, entry, out);
    }
    return out;
  }

  private notify(locale: string): void {
    for (const cb of this.listeners) cb(locale);
  }
}

function lookupByDot(content: Record<string, unknown>, key: string): unknown {
  const [head, ...path] = key.split('.');
  let value: unknown = content[head];
  for (const part of path) {
    if (value === null || typeof value !== 'object') return undefined;
    value = (value as Record<string, unknown>)[part];
  }
  return value;
}

function flatten(prefix: string, value: unknown, out: Record<string, string>): void {
  if (typeof value === 'string') {
    out[prefix] = value;
    return;
  }
  if (Array.isArray(value) || (value !== null && typeof value === 'object')) {
    for (const [k, v] of Object.entries(value as Record<string, unknown>)) {
      flatten(`${prefix}.${k}`, v, out);
    }
  }
}

function interpolate(template: string, params?: Record<string, string | number>): string {
  if (!params) return template;
  return template.replace(/\{([^}]+)\}/g, (m, name: string) => {
    const v = params[name];
    return v === undefined ? m : String(v);
  });
}