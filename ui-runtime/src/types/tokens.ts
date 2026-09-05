import type { ThemeDescriptor } from './descriptor';

export interface DesignToken {
  name: string;
  value: string;
}

export interface TokenFindAllResult {
  theme: ThemeDescriptor;
  tokens: DesignToken[];
}

export interface CssVarsTarget {
  style: Record<string, unknown>;
  /** удалить свойство */
  removeProperty?: (name: string) => void;
}

/** Обёртка для документо-подобного окружения (тестирование, SSR). */
export interface DomLike {
  documentElement: {
    style: CssVarsTarget;
  };
  head?: {
    appendChild: (node: unknown) => void;
  };
  createElement?: (tag: 'link' | 'style') => { setAttribute: (k: string, v: string) => void };
}

export interface TokensContext {
  themeIds: string[];
  fonts: string[];
  tokens: DesignToken[];
}