import { DescriptorValidationError } from '../errors';
import type { ThemeDescriptor, ThemeTokenDef } from '../types/descriptor';

export interface DesignTokensEnv {
  writeCss?: (css: string) => void;
  /** уведомление о списке подключаемых шрифтов */
  onFonts?: (fonts: string[]) => void;
  /** управляемая DOM-среда: инжект <link> для шрифтов */
  document?: {
    createElement: (tagName: 'link') => { rel?: string; type?: string; href?: string };
    head: { appendChild: (node: unknown) => void };
  };
}

export interface DesignTokensOptions {
  scope?: string;
  env?: DesignTokensEnv;
}

function asVar(name: string): string {
  return name.startsWith('--') ? name : `--${name}`;
}

function isDef(v: unknown): v is ThemeTokenDef {
  if (typeof v === 'string') return true;
  if (typeof v !== 'object' || v === null || Array.isArray(v)) return false;
  if ('value' in v) return typeof (v as { value: unknown }).value === 'string';
  if ('ref' in v) return typeof (v as { ref: unknown }).ref === 'string';
  return false;
}

function resolveValue(name: string, def: ThemeTokenDef, tokens: Record<string, ThemeTokenDef>): string {
  if (typeof def === 'string') return def;
  if ('value' in def) return def.value;
  const refName = asVar(def.ref);
  let entry: [string, ThemeTokenDef] | undefined;
  for (const [k, v] of Object.entries(tokens)) {
    if (asVar(k) === refName) {
      entry = [k, v];
      break;
    }
  }
  if (!entry) {
    throw new DescriptorValidationError(`Токен '${name}' ссылается на отсутствующий токен '${def.ref}'`);
  }
  if (entry[0] === name || asVar(entry[0]) === asVar(name)) {
    throw new DescriptorValidationError(`Токен '${name}' ссылается сам на себя`);
  }
  return resolveValue(name, entry[1], tokens);
}

function renderCss(scope: string, flat: Map<string, string>): string {
  const body = [...flat.entries()].map(([k, v]) => `${k}: ${v};`).join(' ');
  return `${scope} { ${body} }`;
}

/** Применяет темы в CSS-переменные (test-spec §13). */
export class DesignTokens {
  private current = new Map<string, string>();
  private scope: string;
  private env?: DesignTokensEnv;

  constructor(opts?: DesignTokensOptions) {
    this.scope = opts?.scope ?? ':root';
    this.env = opts?.env;
  }

  apply(theme: ThemeDescriptor): void {
    const flat = new Map<string, string>();
    for (const [name, def] of Object.entries(theme.tokens)) {
      if (!isDef(def)) {
        throw new DescriptorValidationError(
          `Токен '${name}' темы '${theme.themeId}' должен быть string | { value } | { ref }`,
        );
      }
      flat.set(asVar(name), resolveValue(asVar(name), def, theme.tokens));
    }
    this.current = flat;
    this.env?.writeCss?.(renderCss(this.scope, flat));
    if (theme.fonts?.length) {
      this.attachFonts(theme.fonts);
    }
  }

  get(name: string): string | undefined {
    return this.current.get(asVar(name));
  }

  /** Плоский словарь `--имя → значение` применённой темы (для синхронизации со store). */
  values(): Record<string, string> {
    return Object.fromEntries(this.current);
  }

  private attachFonts(fonts: string[]): void {
    this.env?.onFonts?.(fonts);
    const doc = this.env?.document;
    if (!doc) return;
    for (const font of fonts) {
      const link = doc.createElement('link');
      link.rel = 'stylesheet';
      link.type = 'text/css';
      link.href = font;
      doc.head.appendChild(link);
    }
  }
}