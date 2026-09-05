export type ErrorCode =
  | 'duplicate_registration'
  | 'unknown_entity'
  | 'unknown_provider'
  | 'operation_kind_mismatch'
  | 'route_not_found'
  | 'transport_error'
  | 'descriptor_validation'
  | 'theme_not_found'
  | 'subscription_error'
  | 'scope_error'
  | 'component_not_found'
  | 'cron_parse'
  | 'form_validation'
  | 'locale_unsupported'
  | 'asset_not_found';

/** Базовый класс всех ошибок рантайма; поле `code` — машиночитаемый идентификатор. */
export class RuntimeError extends Error {
  readonly code: ErrorCode;

  constructor(code: ErrorCode, message: string) {
    super(message);
    this.name = new.target.name;
    this.code = code;
  }
}

export class DuplicateRegistrationError extends RuntimeError {
  constructor(message: string) {
    super('duplicate_registration', message);
  }
}

export class UnknownEntityError extends RuntimeError {
  constructor(message: string) {
    super('unknown_entity', message);
  }
}

export class UnknownProviderError extends RuntimeError {
  constructor(message: string) {
    super('unknown_provider', message);
  }
}

export class OperationKindMismatchError extends RuntimeError {
  constructor(message: string) {
    super('operation_kind_mismatch', message);
  }
}

export class RouteNotFoundError extends RuntimeError {
  constructor(message: string) {
    super('route_not_found', message);
  }
}

export interface TransportErrorCause {
  status?: number;
  /** порт/адрес транспорта в cause (test-spec §15#3) */
  port?: number;
  httpError?: boolean;
  graphqlErrors?: unknown[];
  message?: string;
}

export class TransportError extends RuntimeError {
  readonly cause?: TransportErrorCause;

  constructor(message: string, cause?: TransportErrorCause) {
    super('transport_error', message);
    this.cause = cause;
  }
}

export class DescriptorValidationError extends RuntimeError {
  readonly entityId?: string;
  readonly path?: string;

  constructor(message: string, opts?: { entityId?: string; path?: string }) {
    super('descriptor_validation', message);
    this.entityId = opts?.entityId;
    this.path = opts?.path;
  }
}

export class ThemeNotFoundError extends RuntimeError {
  constructor(message: string) {
    super('theme_not_found', message);
  }
}

export class SubscriptionError extends RuntimeError {
  constructor(message: string) {
    super('subscription_error', message);
  }
}

export class ScopeError extends RuntimeError {
  constructor(message: string) {
    super('scope_error', message);
  }
}

export class ComponentNotFoundError extends RuntimeError {
  constructor(message: string) {
    super('component_not_found', message);
  }
}

export class CronParseError extends RuntimeError {
  constructor(message: string) {
    super('cron_parse', message);
  }
}

export class FormValidationError extends RuntimeError {
  constructor(message: string) {
    super('form_validation', message);
  }
}

export class LocaleUnsupportedError extends RuntimeError {
  constructor(message: string) {
    super('locale_unsupported', message);
  }
}

export class AssetNotFoundError extends RuntimeError {
  constructor(message: string) {
    super('asset_not_found', message);
  }
}

/** Маршаллит ошибку во внешний слой: `{ code, message }`. */
export function toErrorCode(err: unknown): { code: ErrorCode; message: string } {
  if (err instanceof RuntimeError) {
    return { code: err.code, message: err.message };
  }
  const message = err instanceof Error ? err.message : String(err);
  return { code: 'transport_error', message };
}