import { describe, expect, it } from 'vitest';
import {
  AssetNotFoundError,
  ComponentNotFoundError,
  CronParseError,
  DescriptorValidationError,
  DuplicateRegistrationError,
  FormValidationError,
  LocaleUnsupportedError,
  OperationKindMismatchError,
  RouteNotFoundError,
  RuntimeError,
  ScopeError,
  SubscriptionError,
  ThemeNotFoundError,
  TransportError,
  UnknownEntityError,
  UnknownProviderError,
  toErrorCode,
  type ErrorCode,
} from '../../src/errors';

const CATALOG: ReadonlyArray<{ code: ErrorCode; make: () => RuntimeError }> = [
  { code: 'duplicate_registration', make: () => new DuplicateRegistrationError('x') },
  { code: 'unknown_entity', make: () => new UnknownEntityError('x') },
  { code: 'unknown_provider', make: () => new UnknownProviderError('x') },
  { code: 'operation_kind_mismatch', make: () => new OperationKindMismatchError('x') },
  { code: 'route_not_found', make: () => new RouteNotFoundError('x') },
  { code: 'transport_error', make: () => new TransportError('x') },
  { code: 'descriptor_validation', make: () => new DescriptorValidationError('x') },
  { code: 'theme_not_found', make: () => new ThemeNotFoundError('x') },
  { code: 'subscription_error', make: () => new SubscriptionError('x') },
  { code: 'scope_error', make: () => new ScopeError('x') },
  { code: 'component_not_found', make: () => new ComponentNotFoundError('x') },
  { code: 'cron_parse', make: () => new CronParseError('x') },
  { code: 'form_validation', make: () => new FormValidationError('x') },
  { code: 'locale_unsupported', make: () => new LocaleUnsupportedError('x') },
  { code: 'asset_not_found', make: () => new AssetNotFoundError('x') },
];

describe('errors (§15)', () => {
  it('каждый класс Error имеет code из ErrorCatalog', () => {
    for (const entry of CATALOG) {
      const err = entry.make();
      expect(err.code).toBe(entry.code);
      expect(err).toBeInstanceOf(Error);
    }
  });

  it('RuntimeError — базовый класс; instanceof для всех типов', () => {
    for (const entry of CATALOG) {
      expect(entry.make()).toBeInstanceOf(RuntimeError);
    }
    const base = new RuntimeError('scope_error', 'x');
    expect(base).toBeInstanceOf(RuntimeError);
    expect(base).toBeInstanceOf(Error);
  });

  it('TransportError хранит cause (status/порт)', () => {
    const err = new TransportError('http 500', { status: 500, port: 8443 });
    expect(err.cause?.status).toBe(500);
    expect(err.cause?.port).toBe(8443);
    expect(err.code).toBe('transport_error');
  });

  it('toErrorCode маршаллит код для внешнего слоя', () => {
    expect(toErrorCode(new RouteNotFoundError('нет'))).toEqual({ code: 'route_not_found', message: 'нет' });
    expect(toErrorCode(new Error('boom'))).toEqual({ code: 'transport_error', message: 'boom' });
    expect(toErrorCode('boom')).toEqual({ code: 'transport_error', message: 'boom' });
    expect(toErrorCode(42)).toEqual({ code: 'transport_error', message: '42' });
  });
});