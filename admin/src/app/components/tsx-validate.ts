import { transform } from 'sucrase';

/** Минимальный срез реестра для проверки импорт-политики. */
export interface ImportPolicyRegistryEntry {
  id: string;
  isSection: boolean;
}

/**
 * Результат live-валидации TSX (sucrase, лёгкий транслер — типы не проверяются).
 */
export interface TsxDiagnostic {
  ok: boolean;
  line?: number;
  message?: string;
}

/**
 * Возвращает id-ы, на которые исходник ссылается через `@site/components/<id>`
 * (статический `import X from …` и динамический `import("…")`), — зеркало
 * `build.ScanImportSpecifiers` на бэкенде.
 */
export function siteComponentImports(source: string): string[] {
  const re = /from\s*["']@site\/components\/([^"']+)["']|import\(\s*["']@site\/components\/([^"']+)["']\s*\)/g;
  const out: string[] = [];
  let m: RegExpExecArray | null;
  while ((m = re.exec(source)) !== null) {
    const id = (m[1] ?? m[2] ?? '').trim();
    if (id) out.push(id);
  }
  return out;
}

/** Источник объявляет default-экспорт (короткая форма `export default …`). */
export function hasDefaultExport(source: string): boolean {
  return /\bexport\s+default\b/.test(source);
}

/** Нормализует список allowlist: убирает дубли и сортирует. */
export function normalizeAllowlist(ids?: string[]): string[] {
  if (!ids || ids.length === 0) return [];
  return [...new Set(ids.map((s) => s.trim()).filter(Boolean))].sort();
}

/**
 * Проверяет импорт-политику (R10) для одного определения против всего
 * реестра. Возвращает список нарушений (пустой = допустимо); зеркалит
 * `ValidateGraph` бэкенда в UI для живой подсветки.
 */
export function validateImportPolicy(
  source: string,
  registry: ImportPolicyRegistryEntry[],
  isSection: boolean,
  acceptsPageContent: boolean,
  allowedPrimitiveIds: string[] | undefined,
): string[] {
  const errors: string[] = [];
  if (!hasDefaultExport(source)) {
    errors.push('Объявите default-экспорт (`export default …`)');
  }
  if (acceptsPageContent && !isSection) {
    errors.push('acceptsPageContent требует isSection');
  }
  const imports = siteComponentImports(source);
  if (!isSection) {
    if (imports.length > 0) {
      errors.push(`Примитив не может импортировать компоненты сайта: @site/components/${imports[0]}`);
    }
    return errors;
  }
  const byId = new Map(registry.map((r) => [r.id, r]));
  for (const id of imports) {
    const target = byId.get(id);
    if (!target) {
      errors.push(`Импорт @site/components/${id} — нет такого компонента`);
      continue;
    }
    if (target.isSection) {
      errors.push(`Секция не может импортировать секцию @site/components/${id}`);
      continue;
    }
    const allowed = normalizeAllowlist(allowedPrimitiveIds);
    if (!allowed.includes(id)) {
      errors.push(`Импорт @site/components/${id} не включён в allowlist секции`);
    }
  }
  for (const id of normalizeAllowlist(allowedPrimitiveIds)) {
    const target = byId.get(id);
    if (!target) {
      errors.push(`Allowlist ссылается на неизвестный компонент ${id}`);
      continue;
    }
    if (target.isSection) {
      errors.push(`Allowlist ссылается на секцию ${id}; можно только примитивы`);
    }
  }
  return errors;
}

/**
 * Отбрасывает суффикс позиции `(N:M)` из сообщения sucrase.
 */
function cleanMessage(message: string): string {
  return message.replace(/\s*\(\d+:\d+\)\s*$/, '').trim();
}

/**
 * Проверяет TSX-исходник компонента без типизации (решение R5, риск 2 в
 * backend.md: лёгкий транслер в UI, типы — на бэкенде). Пустой/пробельный
 * исходник считается валидным.
 */
export function validateTsx(source: string): TsxDiagnostic {
  if (!source.trim()) return { ok: true };
  try {
    transform(source, { transforms: ['typescript', 'jsx'] });
    return { ok: true };
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err);
    const match = /\((\d+):(\d+)\)$/.exec(message.trim());
    return {
      ok: false,
      line: match ? Number(match[1]) : undefined,
      message: cleanMessage(message) || 'Синтаксическая ошибка',
    };
  }
}