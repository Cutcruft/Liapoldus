import { describe, it, expect, beforeEach } from 'vitest';
import { RuntimeRegistry } from '../../src/core/registry';
import {
  DescriptorValidationError,
  DuplicateRegistrationError,
  UnknownProviderError,
  UnknownEntityError,
  CronParseError,
} from '../../src/errors';
import {
  providerDescriptor1,
  operationDescriptor1,
  operationDescriptor10,
  endpointDescriptor2,
  endpointDescriptor1,
  contractDescriptor,
  routeDescriptor6,
} from './fixtures';

describe('RuntimeRegistry', () => {
  let registry: RuntimeRegistry;

  beforeEach(() => {
    registry = new RuntimeRegistry();
  });

  it('регистрирует provider и резолвит по id', () => {
    registry.register(providerDescriptor1);
    const p = registry.getProvider('cms');
    expect(p).not.toBeNull();
    expect(p!.id).toBe('cms');
    expect(p!.protocol).toBe('http');
    expect(p!.baseUrl).toBe('https://cms.example.com/api');
  });

  it('бросает DuplicateRegistrationError при повторной регистрации', () => {
    registry.register(providerDescriptor1);
    expect(() => registry.register(providerDescriptor1)).toThrow(DuplicateRegistrationError);
  });

  it('бросает DescriptorValidationError на невалидный baseUrl', () => {
    expect(() =>
      registry.register({ ...providerDescriptor1, baseUrl: 'not-a-url' }),
    ).toThrow(DescriptorValidationError);
  });

  it('операция без зарегистрированного provider → UnknownProviderError', () => {
    expect(() => registry.register(operationDescriptor1)).toThrow(UnknownProviderError);
  });

  it('регистрирует операцию и резолвит её вместе с provider', () => {
    registry.register(providerDescriptor1);
    registry.register(operationDescriptor1);
    const op = registry.getOperation('content.get');
    expect(op).not.toBeNull();
    expect(op!.provider.id).toBe('cms');
    expect(op!.cache).toBe('immutable');
    expect(op!.scope).toBe('public');
  });

  it('ws-операция без subscribe.url → DescriptorValidationError', () => {
    registry.register({ kind: 'provider', id: 'ws-bare', protocol: 'ws', baseUrl: 'wss://ws.example.com' });
    expect(() =>
      registry.register({ ...operationDescriptor1, providerId: 'ws-bare', subscribe: undefined }),
    ).toThrow(DescriptorValidationError);
  });

  it('невалидный cron в poll → CronParseError', () => {
    registry.register(providerDescriptor1);
    expect(() =>
      registry.register({
        ...operationDescriptor1,
        poll: { schedule: '0 0 * * *' }, // 5 полей вместо 6
      }),
    ).toThrow(CronParseError);
  });

  it('getOperationFor возвращает операцию по типу и provider', () => {
    registry.register(providerDescriptor1);
    registry.register(operationDescriptor1);
    const op = registry.getOperationFor('query', 'cms');
    expect(op!.id).toBe('content.get');
  });

  it('resolve по бейджеванному id `id#query`', () => {
    registry.register(providerDescriptor1);
    registry.register(operationDescriptor1);
    const op = registry.resolve('query', 'content.get#query');
    expect(op.id).toBe('content.get');
  });

  it('resolve неизвестной операции → UnknownEntityError', () => {
    expect(() => registry.resolve('query', 'missing')).toThrow(UnknownEntityError);
  });

  it('endpoint с валидной server-операцией регистрируется', () => {
    registry.register({ kind: 'provider', id: 'builtin', protocol: 'http' });
    registry.register(operationDescriptor10);
    registry.register(endpointDescriptor2);
    const ep = registry.getEndpoint('contact.submit');
    expect(ep).not.toBeNull();
    expect(ep!.operation.id).toBe('form.submit');
    expect(ep!.method).toBe('POST');
  });

  it('endpoint на публичную операцию → DescriptorValidationError', () => {
    registry.register(providerDescriptor1);
    registry.register(operationDescriptor1);
    expect(() => {
      registry.register({
        kind: 'endpoint',
        id: 'bad.endpoint',
        path: '/x',
        method: 'POST',
        operationId: 'content.get',
      });
    }).toThrow(DescriptorValidationError);
  });

  it('endpoint на несуществующую операцию → UnknownEntityError', () => {
    registry.register({ kind: 'provider', id: 'builtin', protocol: 'http' });
    expect(() =>
      registry.register({
        kind: 'endpoint',
        id: 'fn',
        path: '/fn',
        method: 'GET',
        operationId: 'missing.op',
      }),
    ).toThrow(UnknownEntityError);
  });

  it('endpoint без path → DescriptorValidationError', () => {
    registry.register({ kind: 'provider', id: 'builtin', protocol: 'http' });
    registry.register(operationDescriptor10);
    expect(() =>
      registry.register({ ...endpointDescriptor1, path: '' }),
    ).toThrow(DescriptorValidationError);
  });

  it('drop() очищает все регистрации', () => {
    registry.register(providerDescriptor1);
    registry.register(operationDescriptor1);
    registry.register({ kind: 'provider', id: 'builtin', protocol: 'http' });
    registry.register(operationDescriptor10);
    registry.register(endpointDescriptor1);
    registry.register(routeDescriptor6 as never);
    registry.drop();
    expect(registry.getProvider('cms')).toBeNull();
    expect(registry.getOperation('content.get')).toBeNull();
    expect(registry.getEndpoint('form.submit')).toBeNull();
    expect(registry.routesList).toHaveLength(0);
  });

  it('контракт целиком: все сущности регистрируются', () => {
    for (const d of [
      ...contractDescriptor.protocols,
      ...contractDescriptor.operations,
      ...contractDescriptor.endpoints,
      ...contractDescriptor.routes,
      ...contractDescriptor.themes,
    ]) {
      registry.register(d as never);
    }
    expect(registry.hasProvider('cms')).toBe(true);
    expect(registry.hasOperation('form.submit')).toBe(true);
    expect(registry.hasEndpoint('contact.submit')).toBe(true);
    expect(registry.routesList.length).toBe(5);
    expect(registry.themesList.length).toBe(2);
  });
});