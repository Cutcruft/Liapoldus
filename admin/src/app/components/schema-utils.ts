import type { InferredProp } from '../../runtime';

/**
 * Эвристический авто-вывод props компонента из TSX-исходника (R5; ручные
 * уточнения — R8). Источники, от «надёжного» к «догадкам»:
 *   1. `interface/type Props { ... }` — полные типы;
 *   2. инлайн-тип в сигнатуре `(props: { title: string }) => ...`;
 *   3. деструктуризация параметра `({ title, count = 0 }: ...)` — имена и дефолты.
 */

/** Возвращает тело блока `{...}`, начиная с openIndex (без скобок). */
function extractBlock(source: string, openIndex: number): string {
  let depth = 1;
  for (let i = openIndex + 1; i < source.length; i++) {
    const ch = source[i];
    if (ch === '{') depth++;
    else if (ch === '}') {
      depth--;
      if (depth === 0) return source.slice(openIndex + 1, i);
    }
  }
  return '';
}

/** Разбивает тело объявления на члены по `;` вне скобок. */
function splitMembers(body: string): string[] {
  const parts: string[] = [];
  let depth = 0;
  let cur = '';
  for (const ch of body) {
    if (ch === '{' || ch === '[' || ch === '(') depth++;
    else if (ch === '}' || ch === ']' || ch === ')') depth--;
    if (ch === ';' && depth === 0) {
      parts.push(cur);
      cur = '';
      continue;
    }
    cur += ch;
  }
  if (cur.trim()) parts.push(cur);
  return parts;
}

function stripComments(text: string): string {
  return text
    .replace(/\/\*[\s\S]*?\*\//g, '')
    .replace(/\/\/[^\n]*/g, '')
    .trim();
}

/** Похоже на тип, а не на литерал (`'x'`, `4`, `true`) или значение URL/комментария. */
function looksLikeType(type: string): boolean {
  const t = type.trim();
  if (!t) return false;
  if (/^['"`]/.test(t)) return false;
  if (/^\d/.test(t)) return false;
  if (/^(true|false|null|undefined)$/.test(t)) return false;
  if (/^(https?:\/\/|\/\/)/.test(t)) return false;
  return true;
}

function parseMember(member: string, into: Map<string, InferredProp>): void {
  const clean = stripComments(member);
  if (!clean) return;
  const m = /^(?:readonly\s+)?(?:['"]?)([A-Za-z_$][\w$]*)(?:['"]?)\s*(\??)\s*:\s*(.+)$/.exec(clean);
  if (!m) return;
  const name = m[1];
  if (name === undefined) return;
  const type = (m[3] ?? '').trim();
  if (!looksLikeType(type)) return;
  const required = (m[2] ?? '') !== '?';
  if (required || !into.has(name)) into.set(name, { name, type, required });
}

/** Ищет `interface/type Props { ... }` (скобка принадлежит блоку целиком). */
function parseDeclaredBlocks(source: string, into: Map<string, InferredProp>): void {
  const re = /(?:interface|type)\s+[A-Za-z_$][\w$]*(?:<[^>]*>)?(?:\s*=\s*)?\s*\{/g;
  let m: RegExpExecArray | null;
  while ((m = re.exec(source)) !== null) {
    const open = source.indexOf('{', m.index);
    if (open < 0) continue;
    parseTypeBody(extractBlock(source, open), into);
    re.lastIndex = Math.max(re.lastIndex, open + 1);
  }
}

/** Инлайн-типы вида `(props: { title: string })`: оставляем только аннотации с «типами». */
function parseInlineTypes(source: string, into: Map<string, InferredProp>): void {
  const re = /[A-Za-z_$][\w$]*\s*:\s*\{/g;
  let m: RegExpExecArray | null;
  while ((m = re.exec(source)) !== null) {
    const open = source.indexOf('{', m.index);
    if (open < 0) continue;
    const body = extractBlock(source, open);
    const before = into.size;
    parseTypeBody(body, into);
    if (into.size === before) continue;
    re.lastIndex = Math.max(re.lastIndex, open + 1);
  }
}

function parseTypeBody(body: string, into: Map<string, InferredProp>): void {
  for (const member of splitMembers(body)) parseMember(member, into);
}

/** Дефолты и «голые» имена из `({ a, b = 1 }: Props)` деструктуризации параметра. */
function parseParamDefaults(source: string, into: Map<string, InferredProp>): void {
  const re = /\(\s*\{([\s\S]*?)\}\s*(?::\s*[^)]*)?\s*\)/g;
  let m: RegExpExecArray | null;
  while ((m = re.exec(source)) !== null) {
    const body = m[1] ?? '';
    let depth = 0;
    let cur = '';
    const chunks: string[] = [];
    for (const ch of body) {
      if (ch === '{' || ch === '[' || ch === '(') depth++;
      else if (ch === '}' || ch === ']' || ch === ')') depth--;
      if (ch === ',' && depth === 0) {
        chunks.push(cur);
        cur = '';
        continue;
      }
      cur += ch;
    }
    if (cur.trim()) chunks.push(cur);
    for (const raw of chunks) {
      const clean = raw.trim().replace(/^\.\.\./, '');
      const dm = /^([A-Za-z_$][\w$]*)\s*(\?)?(?:\s*=\s*(.+))?$/.exec(clean);
      if (!dm) continue;
      const name = dm[1];
      if (name === undefined) continue;
      const current = into.get(name);
      const required = (dm[2] ?? '') !== '?';
      const value = (dm[3] ?? '').trim();
      if (current) {
        if (value) current.default = value;
        if (current.type === 'unknown' && required) current.required = required;
      } else {
        const prop: InferredProp = { name, type: 'unknown', required };
        if (value) prop.default = value;
        into.set(name, prop);
      }
    }
  }
}

/** Авто-вывод props из TSX-исходника (порядок появления в коде). */
export function inferProps(source: string): InferredProp[] {
  const byName = new Map<string, InferredProp>();
  parseDeclaredBlocks(source, byName);
  parseInlineTypes(source, byName);
  parseParamDefaults(source, byName);
  return Array.from(byName.values());
}

/** Детектируемое имя компонента из default-экспорта; null — если не нашлось. */
export function inferComponentName(source: string): string | null {
  const namedFn = /export\s+default\s+(?:async\s+)?function\s+([A-Za-z_$][\w$]*)/.exec(source);
  if (namedFn && namedFn[1]) return namedFn[1];
  const namedConst = /export\s+default\s+([A-Za-z_$][\w$]*)\s*=\s*(?:function\b|await\s+|\()/.exec(source);
  if (namedConst && namedConst[1]) return namedConst[1];
  const anonymous = /export\s+default\s+(?:\([^)]*\)\s*=>|function\s*\()/.test(source);
  if (anonymous) {
    const propsType = /(?:interface|type)\s+([A-Za-z_$][\w$]*)\s*\{/.exec(source);
    return propsType?.[1] ?? 'Component';
  }
  return null;
}