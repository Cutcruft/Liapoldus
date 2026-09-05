import {
  DescriptorValidationError,
  DuplicateRegistrationError,
  UnknownEntityError,
  UnknownProviderError,
} from '../errors';
import type {
  Descriptor,
  EndpointDescriptor,
  OperationDescriptor,
  OperationScope,
  OperationType,
  ProviderDescriptor,
  RouteDescriptor,
  ThemeDescriptor,
} from '../types/descriptor';
import { parseCron } from './cron';
import { splitBadgedId } from './descriptor';

export interface ResolvedProvider {
  id: string;
  protocol: ProviderDescriptor['protocol'];
  baseUrl?: string;
  defaults?: ProviderDescriptor['defaults'];
  subscribe?: ProviderDescriptor['subscribe'];
  state: { registered: true };
}

export interface ResolvedOperation {
  id: string;
  provider: ResolvedProvider;
  typeOp: OperationType;
  cache: OperationDescriptor['cache'];
  scope: OperationScope;
  method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE';
  path?: string;
  params?: OperationDescriptor['params'];
  input?: OperationDescriptor['input'];
  output?: OperationDescriptor['output'];
  type?: string;
  ttl?: number;
  poll?: OperationDescriptor['poll'];
  push?: boolean;
  subscribe?: OperationDescriptor['subscribe'];
  headers?: Record<string, string>;
  state: { registered: true };
}

export interface ResolvedEndpoint {
  id: string;
  path: string;
  method: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE';
  operation: ResolvedOperation;
  input?: EndpointDescriptor['input'];
  output?: EndpointDescriptor['output'];
  state: { registered: true };
}

export interface RegisterContext {
  srcZipLike?: unknown;
}

/** Валидирует ссылки между дескрипторами и хранит резолвнутые сущности (§12, §12a). */
export class RuntimeRegistry {
  private providers = new Map<string, ResolvedProvider>();
  private operations = new Map<string, ResolvedOperation>();
  private endpoints = new Map<string, ResolvedEndpoint>();
  private routes: RouteDescriptor[] = [];
  private themes: ThemeDescriptor[] = [];

  register(descriptor: Descriptor, _ctx?: RegisterContext): void {
    switch (this.kindOf(descriptor)) {
      case 'provider':
        this.registerProvider(descriptor as ProviderDescriptor);
        return;
      case 'operation':
        this.registerOperation(descriptor as OperationDescriptor);
        return;
      case 'endpoint':
        this.registerEndpoint(descriptor as EndpointDescriptor);
        return;
      case 'route':
        this.registerRoute(descriptor as RouteDescriptor);
        return;
      case 'theme':
        this.registerTheme(descriptor as ThemeDescriptor);
        return;
      default:
        throw new DescriptorValidationError(`Неизвестный тип дескриптора: ${String((descriptor as Descriptor)?.kind)}`);
    }
  }

  /** kind из `kind`-поля; канонические route/theme без kind распознаются структурно. */
  private kindOf(d: unknown): string {
    if (d && typeof d === 'object') {
      const rec = d as Record<string, unknown>;
      if (typeof rec.kind === 'string') return rec.kind;
      if ('action' in rec && 'matcher' in rec) return 'route';
      if ('themeId' in rec && 'tokens' in rec) return 'theme';
    }
    return 'unknown';
  }

  registerProvider(descriptor: ProviderDescriptor): void {
    if (this.providers.has(descriptor.id)) {
      throw new DuplicateRegistrationError(`Provider '${descriptor.id}' уже зарегистрирован`);
    }
    if (descriptor.baseUrl !== undefined) {
      if (typeof descriptor.baseUrl !== 'string' || !/^[a-z][a-z0-9+.-]*:\/\//i.test(descriptor.baseUrl)) {
        throw new DescriptorValidationError(
          `baseUrl '${String(descriptor.baseUrl)}' не является валидным абсолютным URL`,
          { entityId: descriptor.id, path: 'baseUrl' },
        );
      }
      let url: URL;
      try {
        url = new URL(descriptor.baseUrl);
      } catch {
        throw new DescriptorValidationError(
          `baseUrl '${descriptor.baseUrl}' не является валидным URL`,
          { entityId: descriptor.id, path: 'baseUrl' },
        );
      }
      if (!/^(https?|wss?):$/.test(url.protocol)) {
        throw new DescriptorValidationError(
          `baseUrl '${descriptor.baseUrl}' имеет неподдерживаемый протокол`,
          { entityId: descriptor.id, path: 'baseUrl' },
        );
      }
    }
    const provider: ResolvedProvider = {
      id: descriptor.id,
      protocol: descriptor.protocol,
      state: { registered: true },
    };
    if (descriptor.baseUrl !== undefined) provider.baseUrl = descriptor.baseUrl;
    if (descriptor.defaults) provider.defaults = descriptor.defaults;
    if (descriptor.subscribe) provider.subscribe = descriptor.subscribe;
    this.providers.set(descriptor.id, provider);
  }

  getProvider(id: string): ResolvedProvider | null {
    return this.providers.get(id) ?? null;
  }

  hasProvider(id: string): boolean {
    return this.providers.has(id);
  }

  get providersList(): ResolvedProvider[] {
    return Array.from(this.providers.values());
  }

  registerOperation(descriptor: OperationDescriptor): void {
    if (this.operations.has(descriptor.id)) {
      throw new DuplicateRegistrationError(`Operation '${descriptor.id}' уже зарегистрирована`);
    }
    const provider = this.providers.get(descriptor.providerId);
    if (!provider) {
      throw new UnknownProviderError(`Provider '${descriptor.providerId}' для operation '${descriptor.id}' не зарегистрирован`);
    }
    if (descriptor.scope === undefined) descriptor.scope = 'public';
    if (descriptor.scope !== 'public' && descriptor.scope !== 'server') {
      throw new DescriptorValidationError('scope operation должен быть public|server', { entityId: descriptor.id, path: 'scope' });
    }
    if (descriptor.poll) {
      if (descriptor.poll.schedule) parseCron(descriptor.poll.schedule);
    }
    if (provider.protocol !== 'http' && descriptor.path !== undefined) {
      const subscribe = descriptor.subscribe;
      const url = subscribe?.url ?? provider.subscribe?.url;
      if (provider.protocol !== 'graphql' && !url) {
        throw new DescriptorValidationError(
          `Operation '${descriptor.id}' (${provider.protocol}) требует subscribe.url`,
          { entityId: descriptor.id, path: 'subscribe.url' },
        );
      }
    }
    const op: ResolvedOperation = {
      id: descriptor.id,
      provider,
      typeOp: descriptor.typeOp,
      cache: descriptor.cache,
      scope: descriptor.scope,
      state: { registered: true },
    };
    if (descriptor.method) op.method = descriptor.method;
    if (descriptor.path) op.path = descriptor.path;
    if (descriptor.params) op.params = descriptor.params;
    if (descriptor.input) op.input = descriptor.input;
    if (descriptor.output) op.output = descriptor.output;
    if (descriptor.type) op.type = descriptor.type;
    if (typeof descriptor.ttl === 'number') op.ttl = descriptor.ttl;
    if (descriptor.poll) op.poll = descriptor.poll;
    if (descriptor.push === true) op.push = true;
    if (descriptor.subscribe) op.subscribe = descriptor.subscribe;
    if (descriptor.headers) op.headers = descriptor.headers;
    this.operations.set(descriptor.id, op);
  }

  getOperation(id: string): ResolvedOperation | null {
    return this.operations.get(id) ?? null;
  }

  hasOperation(id: string): boolean {
    return this.operations.has(id);
  }

  getOperationFor(typeOp: OperationType, providerId: string): ResolvedOperation | null {
    for (const op of this.operations.values()) {
      if (op.typeOp === typeOp && op.provider.id === providerId) return op;
    }
    return null;
  }

  /** Ищет по чистому id, либо по бейджеванному `id#query` (§12a). */
  findOperationByIdOrBadge(ref: string): ResolvedOperation | null {
    const direct = this.operations.get(ref);
    if (direct) return direct;
    const { name, badge } = splitBadgedId(ref);
    if (badge) {
      for (const op of this.operations.values()) {
        if (op.id === name && op.typeOp === badge) return op;
      }
    }
    return null;
  }

  resolve(typeOp: OperationType, ref: string): ResolvedOperation {
    const op =
      this.operations.get(ref) ??
      this.getOperationFor(typeOp, ref) ??
      this.findOperationByIdOrBadge(ref);
    if (!op) {
      const provider = this.providers.get(ref);
      const forType = provider ? this.getOperationFor(typeOp, ref) : null;
      if (forType) return forType;
      throw new UnknownEntityError(
        `Operation '${ref}' типа '${typeOp}' не найдена` + (provider ? ` для provider '${ref}'` : ''),
      );
    }
    if (op.typeOp !== typeOp) {
      throw new DescriptorValidationError(
        `Operation '${op.id}' имеет тип '${op.typeOp}', а требуется '${typeOp}'`,
        { entityId: op.id, path: 'typeOp' },
      );
    }
    return op;
  }

  get operationsList(): ResolvedOperation[] {
    return Array.from(this.operations.values());
  }

  registerEndpoint(descriptor: EndpointDescriptor): void {
    if (this.endpoints.has(descriptor.id)) {
      throw new DuplicateRegistrationError(`Endpoint '${descriptor.id}' уже зарегистрирован`);
    }
    if (!descriptor.path || descriptor.path.length === 0) {
      throw new DescriptorValidationError('endpoint требует path', { entityId: descriptor.id, path: 'path' });
    }
    if (!descriptor.method) {
      throw new DescriptorValidationError('endpoint требует method', { entityId: descriptor.id, path: 'method' });
    }
    const operation = this.operations.get(descriptor.operationId);
    if (!operation) {
      throw new UnknownEntityError(`Operation '${descriptor.operationId}' для endpoint '${descriptor.id}' не найдена`);
    }
    if (operation.scope !== 'server') {
      throw new DescriptorValidationError(
        `Endpoint '${descriptor.id}' ссылается на публичную operation '${descriptor.operationId}' — endpoint требует server-операцию`,
        { entityId: descriptor.id, path: 'operationId' },
      );
    }
    const endpoint: ResolvedEndpoint = {
      id: descriptor.id,
      path: descriptor.path,
      method: descriptor.method,
      operation,
      state: { registered: true },
    };
    if (descriptor.input) endpoint.input = descriptor.input;
    if (descriptor.output) endpoint.output = descriptor.output;
    this.endpoints.set(descriptor.id, endpoint);
  }

  getEndpoint(id: string): ResolvedEndpoint | null {
    return this.endpoints.get(id) ?? null;
  }

  hasEndpoint(id: string): boolean {
    return this.endpoints.has(id);
  }

  get endpointsList(): ResolvedEndpoint[] {
    return Array.from(this.endpoints.values());
  }

  registerRoute(descriptor: RouteDescriptor): void {
    const existing = this.routes.find((r) => r.id === descriptor.id);
    if (existing) {
      throw new DuplicateRegistrationError(`Route '${descriptor.id}' уже зарегистрирована`);
    }
    this.routes.push(descriptor);
  }

  get routesList(): RouteDescriptor[] {
    return this.routes.slice();
  }

  registerTheme(descriptor: ThemeDescriptor): void {
    const existing = this.themes.find((t) => t.themeId === descriptor.themeId);
    if (existing) {
      throw new DuplicateRegistrationError(`Theme '${descriptor.themeId}' уже зарегистрирована`);
    }
    this.themes.push(descriptor);
  }

  hasTheme(themeId: string): boolean {
    return this.themes.some((t) => t.themeId === themeId);
  }

  get themesList(): ThemeDescriptor[] {
    return this.themes.slice();
  }

  /** Очищает все регистрации (§12a drop). */
  drop(): void {
    this.providers.clear();
    this.operations.clear();
    this.endpoints.clear();
    this.routes = [];
    this.themes = [];
  }
}