import { useEffect, useMemo, useState, type FormEvent, type ReactNode } from 'react';
import { useParams } from 'react-router-dom';
import { Inline, Stack } from '@liapoldus/ui-kit';
import {
  runOperation,
  type Dependency,
  type EvictResponse,
  type LockedDep,
  type ResolveResult,
  type SiteCacheConfig,
} from '../../runtime';
import { useAdmin } from '../admin-context';
import { useOperation } from '../use-operation';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Field } from '../components/Field';

interface TreeNode {
  key: string;
  name: string;
  version: string;
  hoisted: boolean;
  hasPeers: boolean;
  depth: number;
  children: TreeNode[];
}

/**
 * Собирает дерево из плоского BFS-списка инстансов (depsLock snapshot/resolve).
 * Корень = инстансы, объявленные в топе (requestedBy содержит "site");
 * дети инстанса K = инстансы, у которых K встречается в requestedBy.
 */
function buildTree(deps: LockedDep[]): TreeNode[] {
  const below = new Map<string, TreeNode[]>();
  for (const d of deps) {
    for (const parent of d.requestedBy ?? []) {
      if (parent === 'site') continue;
      if (!below.has(parent)) below.set(parent, []);
      below.get(parent)!.push(createNode(d));
    }
  }

  const used = new Set<string>();
  const mount = (node: TreeNode, depth: number) => {
    node.depth = depth;
    used.add(node.key);
    node.children = (below.get(node.key) ?? []).map((c) => mount(c, depth + 1));
    return node;
  };

  const roots = deps
    .filter((d) => (d.requestedBy ?? []).includes('site'))
    .map((d) => mount(createNode(d), 0));

  // Страховка: если какой-то инстанс не достижим из корней (периметр чипов), не теряем его.
  const reachable = new Set<string>();
  const walk = (n: TreeNode) => {
    reachable.add(n.key);
    for (const c of n.children) walk(c);
  };
  roots.forEach(walk);
  for (const d of deps) {
    if (!reachable.has(d.name + '@' + d.version)) roots.push(mount(createNode(d), 0));
  }
  return roots;
}

function createNode(d: LockedDep): TreeNode {
  return {
    key: d.name + '@' + d.version,
    name: d.name,
    version: d.version,
    hoisted: !!d.hoisted,
    hasPeers: Object.keys(d.peerDependencies ?? {}).length > 0,
    depth: 0,
    children: [],
  };
}

function formatBytes(n: number): string {
  if (n >= 1 << 30) return (n / (1 << 30)).toFixed(2) + ' GiB';
  if (n >= 1 << 20) return (n / (1 << 20)).toFixed(1) + ' MiB';
  if (n >= 1 << 10) return (n / (1 << 10)).toFixed(1) + ' KiB';
  return String(n) + ' B';
}

export function SiteDepsPage() {
  const { siteId = '' } = useParams();
  const { api, t } = useAdmin();

  const deps = useOperation<Dependency[]>('listDependencies', { siteId }, (d: unknown) =>
    Array.isArray(d) ? (d as Dependency[]) : [],
  );
  const cache = useOperation<SiteCacheConfig>('getCacheConfig', { siteId }, (d: unknown) =>
    typeof d === 'object' && d !== null ? (d as SiteCacheConfig) : { maxDepsBytes: 0 },
  );

  const [name, setName] = useState('');
  const [spec, setSpec] = useState('');
  const [addError, setAddError] = useState('');
  const [adding, setAdding] = useState(false);

  const [resolved, setResolved] = useState<ResolveResult | null>(null);
  const [resolving, setResolving] = useState(false);
  const [resolveError, setResolveError] = useState('');
  const [expandedParent, setExpandedParent] = useState<Set<string>>(new Set());

  const [limitText, setLimitText] = useState('0');
  const [cacheSave, setCacheSave] = useState<'idle' | 'saving' | 'saved' | 'failed'>('idle');
  const [cacheError, setCacheError] = useState('');
  const [evicting, setEvicting] = useState(false);
  const [evictResult, setEvictResult] = useState<string>('');

  const [limitLoaded, setLimitLoaded] = useState(false);
  useEffect(() => {
    if (!limitLoaded && cache.state.status === 'success') {
      setLimitText(String(cache.state.data.maxDepsBytes ?? 0));
      setLimitLoaded(true);
    }
  }, [limitLoaded, cache.state]);

  const tree = useMemo(
    () => (resolved ? buildTree(resolved.deps) : []),
    [resolved],
  );

  const add = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    if (!name.trim() || !spec.trim()) {
      setAddError(t('common.required'));
      return;
    }
    setAdding(true);
    setAddError('');
    const res = await runOperation(api, 'addDependency', { siteId, name: name.trim(), spec: spec.trim() }, t);
    setAdding(false);
    if (!res.ok) {
      setAddError(res.detail);
      return;
    }
    setName('');
    setSpec('');
    setResolved(null);
    deps.reload();
  };

  const remove = async (dep: Dependency) => {
    const res = await runOperation(api, 'removeDependency', { siteId, name: dep.name }, t);
    if (!res.ok) {
      setAddError(res.detail);
      return;
    }
    setResolved(null);
    deps.reload();
  };

  const resolve = async () => {
    setResolving(true);
    setResolveError('');
    const res = await runOperation(api, 'resolveDependencies', { siteId }, t);
    setResolving(false);
    if (!res.ok) {
      setResolveError(res.detail);
      setResolved(null);
      return;
    }
    setResolved(res.data as ResolveResult);
  };

  const toggle = (key: string) =>
    setExpandedParent((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });

  const saveLimit = async () => {
    const value = Math.max(0, Number(limitText) || 0);
    setCacheSave('saving');
    setCacheError('');
    const res = await runOperation(api, 'updateCacheConfig', { siteId, limitBytes: value }, t);
    if (res.ok) {
      setCacheSave('saved');
      cache.reload();
    } else {
      setCacheSave('failed');
      setCacheError(res.detail);
    }
  };

  const evict = async () => {
    setEvicting(true);
    setEvictResult('');
    const res = await runOperation(api, 'evictCache', { siteId }, t);
    setEvicting(false);
    if (!res.ok) {
      setEvictResult(t('deps.cache.evict.failed', { detail: res.detail }));
      return;
    }
    const body = res.data as EvictResponse | null;
    const n = body?.evicted ?? 0;
    const bytes = body?.evictedBytes ?? 0;
    setEvictResult(n === 0 ? t('deps.cache.saved') : t('deps.cache.evicted', { n, bytes: formatBytes(bytes) }));
  };

  const cacheConfig = cache.state.status === 'success' ? cache.state.data : { maxDepsBytes: 0 };

  const declared = deps.state.status === 'success' ? deps.state.data : [];

  return (
    <Stack gap={6} pad={4}>
      <div>
        <h1 className="text-lg font-medium">{t('deps.title')}</h1>
        <p className="text-sm text-neutral-500">{t('deps.subtitle')}</p>
      </div>

      <section className="rounded-lg border border-neutral-200 p-4">
        <Stack gap={4}>
          <Inline justify="between" align="center">
            <h2 className="text-sm font-medium text-neutral-600">{t('deps.add')}</h2>
          </Inline>
          {deps.state.status === 'loading' && <p className="text-sm text-neutral-400">{t('deps.loading')}</p>}
          {deps.state.status === 'error' && (
            <Inline gap={2} align="center">
              <p className="text-sm text-red-600">{t('deps.error', { detail: deps.state.detail })}</p>
              <Button type="button" variant="outline" size="sm" onClick={deps.reload}>
                {t('deps.reload')}
              </Button>
            </Inline>
          )}
          {deps.state.status === 'success' && (
            <>
              {declared.length === 0 && <p className="text-sm text-neutral-400">{t('deps.none')}</p>}
              {declared.map((dep) => (
                <Inline key={dep.name} gap={2} justify="between" align="center" className="rounded-md border border-neutral-100 px-3 py-2">
                  <span className="text-sm">
                    <code>{dep.name}</code> <span className="text-neutral-400">{dep.spec}</span>
                    {dep.resolvedVersion ? <span className="ml-1 text-neutral-500">→ {dep.resolvedVersion}</span> : null}
                  </span>
                  <Button type="button" variant="outline" size="sm" onClick={() => void remove(dep)}>
                    {t('deps.remove')}
                  </Button>
                </Inline>
              ))}

              <form onSubmit={(e) => void add(e)} className="mt-2 border-t border-neutral-100 pt-3">
                <Inline gap={3} align="end">
                  <Field label={t('deps.name')}>
                    <Input
                      aria-label={t('deps.name')}
                      placeholder={t('deps.name.placeholder')}
                      value={name}
                      onChange={(e) => setName(e.target.value)}
                    />
                  </Field>
                  <Field label={t('deps.spec')}>
                    <Input
                      aria-label={t('deps.spec')}
                      placeholder={t('deps.spec.placeholder')}
                      value={spec}
                      onChange={(e) => setSpec(e.target.value)}
                    />
                  </Field>
                  <Button type="submit" disabled={adding}>
                    {t('deps.add')}
                  </Button>
                </Inline>
                {addError && <p className="mt-2 text-sm text-destructive">{addError}</p>}
              </form>
            </>
          )}

          <Inline gap={2}>
            <Button type="button" onClick={() => void resolve()} disabled={resolving}>
              {resolving ? t('deps.resolving') : t('deps.resolve')}
            </Button>
          </Inline>
          {resolveError && <p className="text-sm text-destructive">{t('deps.resolve.error', { detail: resolveError })}</p>}
        </Stack>
      </section>

      {resolved && (
        <section className="rounded-lg border border-neutral-200 p-4">
          <Stack gap={3}>
            <Inline justify="between" align="center">
              <div>
                <h2 className="text-sm font-medium text-neutral-600">{t('deps.tree')}</h2>
                <p className="text-xs text-neutral-400">{t('deps.tree.hint')}</p>
              </div>
            </Inline>
            {tree.length === 0 ? (
              <p className="text-sm text-neutral-400">{t('deps.resolve.none')}</p>
            ) : (
              <ul className="font-mono text-sm">
                {tree.map((node) => (
                  <TreeNodeRow key={node.key} node={node} expanded={expandedParent} onToggle={toggle} t={t} />
                ))}
              </ul>
            )}
          </Stack>
        </section>
      )}

      <section className="rounded-lg border border-neutral-200 p-4">
        <Stack gap={3}>
          <div>
            <h2 className="text-sm font-medium text-neutral-600">{t('deps.cache.title')}</h2>
            <p className="text-xs text-neutral-400">{t('deps.cache.hint')}</p>
          </div>
          {cache.state.status === 'loading' && <p className="text-sm text-neutral-400">{t('deps.loading')}</p>}
          {cache.state.status === 'error' && (
            <p className="text-sm text-red-600">{t('deps.error', { detail: cache.state.detail })}</p>
          )}
          {cache.state.status === 'success' && (
            <>
              {cacheConfig.maxDepsBytes === 0 ? (
                <p className="text-sm text-neutral-400">{t('deps.cache.unset')}</p>
              ) : (
                <p className="text-sm text-neutral-500">{formatBytes(cacheConfig.maxDepsBytes)}</p>
              )}
              <Inline gap={2} align="end">
                <Field label={t('deps.cache.limit')}>
                  <Input
                    aria-label={t('deps.cache.limit')}
                    inputMode="numeric"
                    type="number"
                    min={0}
                    value={limitText}
                    onChange={(e) => {
                      setLimitText(e.target.value);
                      setCacheSave('idle');
                    }}
                  />
                </Field>
                <Button type="button" variant="outline" onClick={() => void saveLimit()} disabled={cacheSave === 'saving'}>
                  {cacheSave === 'saving' ? t('deps.cache.saving') : t('deps.cache.save')}
                </Button>
                <Button type="button" variant="destructive" onClick={() => void evict()} disabled={evicting}>
                  {evicting ? t('deps.cache.evicting') : t('deps.cache.evict')}
                </Button>
              </Inline>
              {cacheSave === 'saved' && <p className="text-sm text-green-600">{t('deps.cache.saved')}</p>}
              {cacheSave === 'failed' && <p className="text-sm text-destructive">{t('deps.cache.failed', { detail: cacheError })}</p>}
              {evictResult && <p className="text-sm text-neutral-500">{evictResult}</p>}
            </>
          )}
        </Stack>
      </section>
    </Stack>
  );
}

interface TreeNodeProps {
  node: TreeNode;
  expanded: Set<string>;
  onToggle: (key: string) => void;
  t: import('../../runtime').Translate;
}

function TreeNodeRow({ node, expanded, onToggle, t }: TreeNodeProps) {
  const { depth, key, name, version, hoisted, hasPeers, children } = node;
  const hasChildren = children.length > 0;
  const open = expanded.has(key);
  return (
    <li>
      <div className="flex items-center gap-2 py-0.5" style={{ paddingLeft: depth * 16 }}>
        {hasChildren ? (
          <button
            type="button"
            className="w-4 text-left text-neutral-400"
            aria-expanded={open}
            onClick={() => onToggle(key)}
          >
            {open ? '▾' : '▸'}
          </button>
        ) : (
          <span className="w-4 text-neutral-300">·</span>
        )}
        <code>{name}</code>
        <span className="text-neutral-400">{version}</span>
        {hoisted && (
          <Badge className="border-green-200 bg-green-50 text-green-700">{t('deps.badge.hoisted')}</Badge>
        )}
        {!hoisted && (
          <Badge className="border-neutral-200 bg-neutral-50 text-neutral-500">{t('deps.badge.nested')}</Badge>
        )}
        {hasPeers && <Badge className="border-amber-200 bg-amber-50 text-amber-700">{t('deps.badge.peer')}</Badge>}
      </div>
      {open && hasChildren && (
        <ul>
          {children.map((c) => (
            <TreeNodeRow key={c.key} node={c} expanded={expanded} onToggle={onToggle} t={t} />
          ))}
        </ul>
      )}
    </li>
  );
}

function Badge({ className, children }: { className: string; children: ReactNode }) {
  return <span className={`rounded-full border px-1.5 py-0.5 text-[10px] uppercase ${className}`}>{children}</span>;
}