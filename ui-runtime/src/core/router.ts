import { RouteNotFoundError } from '../errors';
import type { ResolvedRoute, RouteDescriptor } from '../types/descriptor';
import { applyTargetTemplate, appendQuery, createMatcher, splitPath } from './matcher';
import type { RuntimeStore } from './store';

export interface RouterEnv {
  /** полный переход при serveAsset (§12#7) */
  window?: { location: { assign(url: string): void } };
  basePath?: string;
}

export interface RouterEventMap {
  renderPage: { pageId: string };
  route: { route: ResolvedRoute };
  external: { url: string };
}

export type RouterEvent = { type: keyof RouterEventMap } & RouterEventMap[keyof RouterEventMap];

type RouterListener<K extends keyof RouterEventMap> = (payload: RouterEventMap[K]) => void;

const MAX_REDIRECT_DEPTH = 10;

export class Router {
  private routes: RouteDescriptor[] = [];
  private listeners = new Map<keyof RouterEventMap, Array<(p: never) => void>>();

  constructor(
    private store: RuntimeStore,
    private env?: RouterEnv,
  ) {}

  addRoutes(descriptors: RouteDescriptor[]): void {
    this.routes.push(...descriptors);
  }

  replaceRoutes(descriptors: RouteDescriptor[]): void {
    this.routes = descriptors.slice();
  }

  /** Та же функция матчинга, что у edge (общий createMatcher). */
  match(path: string): ResolvedRoute | null {
    return createMatcher(this.routes)(path);
  }

  on<K extends keyof RouterEventMap>(event: K, listener: RouterListener<K>): () => void {
    const list = this.listeners.get(event) ?? [];
    list.push(listener as (p: never) => void);
    this.listeners.set(event, list);
    return () => {
      this.listeners.set(
        event,
        (this.listeners.get(event) ?? []).filter((l) => l !== (listener as (p: never) => void)),
      );
    };
  }

  navigate(to: string): void {
    const resolved = this.match(to);
    if (!resolved) {
      throw new RouteNotFoundError(`Маршрут не найден: ${to}`);
    }
    this.dispatch(resolved, to, 0);
  }

  private emit<K extends keyof RouterEventMap>(event: { type: K } & RouterEventMap[K]): void {
    for (const listener of this.listeners.get(event.type) ?? []) listener(event as never);
  }

  private dispatch(resolved: ResolvedRoute, to: string, depth: number): void {
    if (depth > MAX_REDIRECT_DEPTH) {
      throw new RouteNotFoundError(`Слишком много редиректов при навигации: ${to}`);
    }
    const action = resolved.route.action;
    if (action.type === 'renderPage') {
      this.store.getState().setRoute(resolved);
      this.emit({ type: 'route', route: resolved });
      this.emit({ type: 'renderPage', pageId: action.pageId });
      return;
    }
    if (action.type === 'serveAsset') {
      this.emit({ type: 'external', url: to });
      this.env?.window?.location.assign(to);
      return;
    }
    const { pathname, query } = splitPath(to);
    const target = applyTargetTemplate(action.target, resolved.route.matcher, pathname);
    const url = action.keepQuery ? appendQuery(target, query) : target;
    const next = this.match(url);
    if (!next) {
      throw new RouteNotFoundError(`Цель редиректа не найдена: ${url}`);
    }
    this.dispatch(next, url, depth + 1);
  }
}