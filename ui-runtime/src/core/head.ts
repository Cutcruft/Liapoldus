import type { SiteHead } from '../types/descriptor';
import type { PageHead } from '../types/page';

/**
 * Head-резолв (R10 P1): клиент мержит site-дефолты (Contract.head) с raw
 * per-page head (PageDescriptor/Declaration.head). Скалярные поля страницы
 * перекрывают site-дефолты; `og`/`meta` мержатся по ключу (site → page).
 */

export interface ResolvedHead {
  /** `<title>`: site.titleTemplate с подстановкой {title}; иначе page.title/site */
  title?: string;
  description?: string;
  robots?: string;
  canonical?: string;
  faviconAssetId?: string;
  og: Record<string, string>;
  meta: Record<string, string>;
}

function resolveTitle(site: SiteHead | undefined, page: PageHead | undefined): string | undefined {
  const pageTitle = page?.title;
  const siteTemplate = site?.titleTemplate;
  // Шаблон форматирует page title (подстановка {title}); если шаблона нет или
  // в нём нет плейсхолдера, page title перекрывает site-дефолт целиком.
  if (pageTitle && siteTemplate?.includes('{title}')) {
    return siteTemplate.replace(/\{title\}/g, pageTitle);
  }
  if (pageTitle) return pageTitle;
  if (siteTemplate) return siteTemplate;
  return undefined;
}

/**
 * Мержит site-дефолты и per-page override в единый head для вывода в document.
 * Оба аргумента опциональны; результат — объект с заполненными og/meta всегда.
 */
export function resolvePageHead(site: SiteHead | undefined, page: PageHead | undefined): ResolvedHead {
  const og = { ...(site?.og ?? {}), ...(page?.og ?? {}) };
  const meta = { ...(site?.meta ?? {}), ...(page?.meta ?? {}) };
  return {
    ...(resolveTitle(site, page) !== undefined ? { title: resolveTitle(site, page) } : {}),
    ...(page?.description ?? site?.description ? { description: page?.description ?? site?.description } : {}),
    ...(page?.robots ? { robots: page.robots } : {}),
    ...(page?.canonical ? { canonical: page.canonical } : {}),
    ...(site?.faviconAssetId ? { faviconAssetId: site.faviconAssetId } : {}),
    og,
    meta,
  };
}

/** Есть ли хоть одно head-значение для вывода (иначе ничего не трогаем). */
export function resolvedHeadHasContent(head: ResolvedHead): boolean {
  return (
    head.title !== undefined ||
    head.description !== undefined ||
    head.robots !== undefined ||
    head.canonical !== undefined ||
    head.faviconAssetId !== undefined ||
    Object.keys(head.og).length > 0 ||
    Object.keys(head.meta).length > 0
  );
}
