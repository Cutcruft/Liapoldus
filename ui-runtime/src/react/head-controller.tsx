import { useEffect, useRef } from 'react';
import { resolvePageHead, resolvedHeadHasContent, type ResolvedHead } from '../core/head';
import { useRuntime } from './context';
import { useTree } from './hooks';

/** Минимальный инъектируемый DOM-интерфейс для head-применения. */
export interface HeadDocument {
  title: string;
  createElement(tag: string): HeadDomElement;
  head: {
    querySelector(selectors: string): HeadDomElement | null;
    appendChild(node: HTMLElement): void;
  };
}

export interface HeadDomElement extends HTMLElement {}

export interface HeadControllerProps {
  /** DOM для записи (тесты: инъекция). Default — глобальный `document`. */
  document?: HeadDocument;
  /** favicon assetId → URL; default — `/assets/{id}`. */
  resolveFavicon?: (assetId: string) => string | undefined;
}

type OwnedMap = Map<string, HTMLElement>;

const selectorOf = (kind: 'meta:name' | 'meta:property' | 'link', key: string): string =>
  kind === 'meta:name' ? `meta[name="${key}"]` : kind === 'meta:property' ? `meta[property="${key}"]` : `link[rel="${key}"]`;

/**
 * PURE: применяет resolved head к document.head. `owned` — карта узлов,
 * созданных прежними применениями (key → element): существующие обновляются,
 * отсутствующие — создаются, больше ненужные — удаляются. Возвращает обновлённую
 * карту, которую должен вернуть следующий вызов.
 */
export function applyHead(
  doc: HeadDocument,
  head: Partial<ResolvedHead>,
  owned: OwnedMap,
  opts: { resolveFavicon?: (assetId: string) => string | undefined } = {},
): OwnedMap {
  const next = new Map<string, HTMLElement>();

  const upsertMeta = (kind: 'meta:name' | 'meta:property', key: string, content: string): void => {
    const k = `${kind}:${key}`;
    let el = owned.get(k) ?? doc.head.querySelector(selectorOf(kind, key));
    if (!el) {
      el = doc.createElement('meta');
      doc.head.appendChild(el);
    }
    if (kind === 'meta:name') el.setAttribute('name', key);
    else el.setAttribute('property', key);
    el.setAttribute('content', content);
    next.set(k, el);
  };

  const upsertLink = (rel: string, href: string): void => {
    const k = `link:${rel}`;
    let el = owned.get(k) ?? doc.head.querySelector(`link[rel="${rel}"]`);
    if (!el) {
      el = doc.createElement('link');
      doc.head.appendChild(el);
    }
    el.setAttribute('rel', rel);
    el.setAttribute('href', href);
    next.set(k, el);
  };

  if (head.title !== undefined && head.title !== '') doc.title = head.title;
  if (head.description !== undefined) upsertMeta('meta:name', 'description', head.description);
  if (head.robots !== undefined) upsertMeta('meta:name', 'robots', head.robots);
  if (head.canonical !== undefined) upsertLink('canonical', head.canonical);
  if (head.faviconAssetId !== undefined) {
    const url = opts.resolveFavicon?.(head.faviconAssetId) ?? `/assets/${head.faviconAssetId}`;
    if (url !== '') upsertLink('icon', url);
  }
  for (const [k, v] of Object.entries(head.og ?? {})) upsertMeta('meta:property', `og:${k}`, v);
  for (const [k, v] of Object.entries(head.meta ?? {})) upsertMeta('meta:name', k, v);

  // Убираем узлы, созданные прошлым применением, но не нужные в новом head.
  for (const [k, el] of owned) {
    if (!next.has(k)) el.remove();
  }
  return next;
}

/**
 * HeadController (R10 P1): применяет effective head (site defaults + per-page
 * override) к document. Пересобирается при смене страницы/head; при анмаунте
 * удаляет созданные им узлы (не трогает то, что существовало до него).
 */
export function HeadController(props: HeadControllerProps = {}): null {
  const runtime = useRuntime();
  const tree = useTree();
  const owned = useRef<OwnedMap>(new Map());
  const faviconRef = useRef(props.resolveFavicon);
  faviconRef.current = props.resolveFavicon;

  const head = tree?.head;
  const siteHead = runtime.siteHead;
  const doc = props.document ?? (typeof document !== 'undefined' ? document : null);

  useEffect(() => {
    if (!doc) return;
    const resolved = resolvePageHead(siteHead, head);
    if (!resolvedHeadHasContent(resolved)) return;
    owned.current = applyHead(doc, resolved, owned.current, {
      resolveFavicon: faviconRef.current,
    });
  }, [doc, siteHead, head]);

  useEffect(() => {
    const ownedNow = owned.current;
    return () => {
      for (const el of ownedNow.values()) el.remove();
    };
  }, []);

  return null;
}