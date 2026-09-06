import type { TreeDeclaration } from '../types/tree';
import type { FetchLike } from './transport/transport';

/**
 * Постраничная доставка (Этап 5 п.4, спека §16).
 *
 * Контракт:
 *  - сборка публикует манифест `/build/{site}/{env}/{version}/manifest.json` и
 *    код-сплит чанки `.../pages/{pageId}.js`;
 *  - каждый чанк сам регистрирует свои компоненты (ComponentRegistry) и дерево
 *    страницы через `registerPage(pageId, tree)` — статический реестр ниже;
 *  - корневой entry.js (shell) только монтируется; чанк домашней страницы
 *    помечается в index.html <link rel=modulepreload>.
 */

/** Зарегистрированные деревья страниц (чанки кладут сюда; mount читает отсюда). */
const pageTrees = new Map<string, TreeDeclaration>();

/** Точка входа чанка: регистрирует дерево своей страницы в статическом реестре. */
export function registerPage(pageId: string, tree: TreeDeclaration): void {
  pageTrees.set(pageId, tree);
}

export function getPageTree(pageId: string): TreeDeclaration | undefined {
  return pageTrees.get(pageId);
}

export function hasPageTree(pageId: string): boolean {
  return pageTrees.has(pageId);
}

/** Ссылка на страницу из манифеста сборки (Go: build.PageRef). */
export interface PageRef {
  pageId: string;
  chunk: string;
  definitions?: string[];
}

/** Манифест сборки (Go: build.Manifest, camelCase JSON). */
export interface BuildManifest {
  shared: Record<string, string>;
  deps: Record<string, { name: string; version: string; publicArtifact?: string }>;
  externals: string[];
  styles: string[];
  pages: PageRef[];
  homePage: string;
}

/** Парсер манифеста сборки (вандал-независимый: только camelCase поле). */
export function parseManifest(raw: string): BuildManifest {
  const data = JSON.parse(raw) as Record<string, unknown>;
  const pages = Array.isArray(data['pages'])
    ? (data['pages'] as Record<string, unknown>[]).map((p) => ({
        pageId: String(p['pageId'] ?? ''),
        chunk: String(p['chunk'] ?? ''),
        definitions: Array.isArray(p['definitions']) ? (p['definitions'] as string[]) : [],
      }))
    : [];
  return {
    shared: (data['shared'] as Record<string, string> | undefined) ?? {},
    deps: (data['deps'] as Record<string, { name: string; version: string }> | undefined) ?? {},
    externals: Array.isArray(data['externals']) ? (data['externals'] as string[]) : [],
    styles: Array.isArray(data['styles']) ? (data['styles'] as string[]) : [],
    pages,
    homePage: String(data['homePage'] ?? ''),
  };
}

export type ChunkImporter = (chunkUrl: string) => Promise<unknown>;
export type PageLoaderListener = () => void;

const defaultImporter: ChunkImporter = (chunkUrl) => import(chunkUrl);

/**
 * Догрузчик код-сплит чанков (§16#7): манифест → URL чанка → import() с
 * кэшем (одна загрузка на pageId). Отключается (buildRoot = null), когда у
 * сайта нет раздаваемой сборки — тогда дерево приходит из контракта/dev-WS.
 */
export class PageLoader {
  private manifestPromise?: Promise<BuildManifest | null>;
  private chunks = new Map<string, Promise<boolean>>();
  private listeners = new Set<PageLoaderListener>();

  constructor(
    private buildRoot: string | null,
    private fetchLike?: FetchLike,
    private importer: ChunkImporter = defaultImporter,
  ) {}

  get enabled(): boolean {
    return this.buildRoot !== null;
  }

  /** Манифест сборки (кэш на один экземпляр). */
  manifest(): Promise<BuildManifest | null> {
    if (!this.enabled) return Promise.resolve(null);
    if (!this.manifestPromise) {
      this.manifestPromise = this.fetchManifest();
    }
    return this.manifestPromise;
  }

  onLoaded(listener: PageLoaderListener): () => void {
    this.listeners.add(listener);
    return () => {
      this.listeners.delete(listener);
    };
  }

  /** URL чанка страницы (аналог modulepreload из index.html). */
  async chunkUrl(pageId: string): Promise<string | null> {
    if (!this.enabled) return null;
    const manifest = await this.manifest();
    if (!manifest) return null;
    const ref = manifest.pages.find((p) => p.pageId === pageId);
    return ref ? `${this.buildRoot}/${ref.chunk}` : null;
  }

  /** Загружает чанк страницы и ждёт его (регистрация компонентов + дерева). */
  load(pageId: string): Promise<boolean> {
    if (!this.enabled) return Promise.resolve(false);
    const existing = this.chunks.get(pageId);
    if (existing) return existing;
    const pending = this.importChunk(pageId);
    this.chunks.set(pageId, pending);
    return pending;
  }

  /** Домашняя страница (стартовый чанк из манифеста). */
  async ensureHome(): Promise<boolean> {
    const manifest = await this.manifest();
    if (!manifest || !manifest.homePage) return false;
    return this.load(manifest.homePage);
  }

  private async fetchManifest(): Promise<BuildManifest | null> {
    if (!this.buildRoot) return null;
    const fetchLike = this.fetchLike ?? globalThis.fetch;
    if (typeof fetchLike !== 'function') return null;
    const res = await fetchLike(`${this.buildRoot}/manifest.json`);
    if (!res.ok) return null;
    const text = await res.text();
    try {
      return parseManifest(text);
    } catch {
      return null;
    }
  }

  private async importChunk(pageId: string): Promise<boolean> {
    if (!this.buildRoot) return false;
    const manifest = await this.manifest();
    if (!manifest) return false;
    const ref = manifest.pages.find((p) => p.pageId === pageId);
    if (!ref) return false;
    try {
      await this.importer(`${this.buildRoot}/${ref.chunk}`);
      this.emit();
      return true;
    } catch {
      this.chunks.delete(pageId);
      return false;
    }
  }

  private emit(): void {
    for (const listener of this.listeners) listener();
  }
}

/**
 * Корень раздачи сборки: берётся из явной опции, выводится из URL страницы
 * (`/build/{site}/{env}/{version}/…`), иначе — `${origin}/build/…` с версией из
 * контракта (`latest`, когда versionId не задан).
 */
export function resolveBuildBase(
  origin: string | undefined,
  siteId: string,
  environment: string,
  versionId?: string,
  explicit?: string,
): string | null {
  if (explicit) return explicit.replace(/\/+$/, '');
  const path = typeof location !== 'undefined' ? location.pathname : '';
  const m = path.match(/^\/build\/([^/]+)\/([^/]+)\/([^/]+)(?:\/|$)/);
  if (m && m[1] === siteId && m[2] === environment) {
    return `${location.origin}/build/${m[1]}/${m[2]}/${m[3]}`;
  }
  if (!origin) return null;
  return `${origin}/build/${siteId}/${environment}/${versionId ?? 'latest'}`;
}