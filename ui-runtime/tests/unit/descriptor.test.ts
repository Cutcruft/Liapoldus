import { describe, it, expect } from 'vitest';
import {
  RuntimeError,
  toErrorCode,
  DescriptorValidationError,
  UnknownProviderError,
  TransportError,
} from '../../src/errors';
import {
  parseDescriptors,
  parseContractJSON,
  validateProviderDescriptor,
  validateOperationDescriptor,
} from '../../src/core/descriptor';
import { contractJson } from './fixtures';

describe('errors', () => {
  it('несёт машиночитаемый code', () => {
    const err = new UnknownProviderError('нет провайдера');
    expect(err).toBeInstanceOf(RuntimeError);
    expect(err.code).toBe('unknown_provider');
  });

  it('toErrorCode маршаллит RuntimeError и оборачивает неизвестные', () => {
    expect(toErrorCode(new DescriptorValidationError('bad')).code).toBe('descriptor_validation');
    expect(toErrorCode(new TypeError('boom')).code).toBe('transport_error');
    expect(toErrorCode('plain').message).toBe('plain');
  });

  it('TransportError несёт cause', () => {
    const err = new TransportError('500', { status: 500, httpError: true });
    expect(err.code).toBe('transport_error');
    expect(err.cause?.status).toBe(500);
  });
});

describe('descriptor parse (JSON)', () => {
  it('парсит контракт из JSON (с комменダри и trailing-запятой)', () => {
    const parsed = parseDescriptors(contractJson);
    expect(parsed.contract.siteId).toBe('interactive-content');
    expect(parsed.providers.length).toBe(6);
    expect(parsed.operations.length).toBe(10);
    expect(parsed.endpoints.length).toBe(2);
    expect(parsed.routes.length).toBe(5);
    expect(parsed.themes.length).toBe(2);
    expect(parsed.contract.capabilities.formSubmissions).toBe(true);
    expect(parsed.contract.enabledChannels.ws).toBe(true);
  });

  it('допускает line-комментарии и trailing-запятые', () => {
    const json = `{
      "siteId": "s", "environment": "prod", "version": "1", "locale": "ru",
      "providers": [
        { "type": "provider","id": "a", "protocol": "http", // встроенный
        }
      ],
      "operations": [],
      "endpoints": [],
      "routes": [],
      "themes": [],
    }`;
    const parsed = parseDescriptors(json);
    expect(parsed.providers).toHaveLength(1);
    expect(parsed.providers[0].id).toBe('a');
  });

  it('бросает DescriptorValidationError на не-JSON', () => {
    expect(() => parseContractJSON('{ nope')).toThrow(DescriptorValidationError);
  });

  it('бросает при неизвестном протоколе', () => {
    expect(() =>
      validateProviderDescriptor({ kind: 'provider', id: 'x', protocol: 'telnet' }),
    ).toThrow(DescriptorValidationError);
  });

  it('бросает при неверном cache-полисе', () => {
    expect(() =>
      validateOperationDescriptor({ kind: 'operation', id: 'o', typeOp: 'query', providerId: 'p', cache: 'forever' }),
    ).toThrow(DescriptorValidationError);
  });

  it('резолвит короткие имена и type=content привязку', () => {
    const parsed = parseDescriptors(
      JSON.stringify({
        siteId: 's', environment: 'prod', version: '1', locale: 'ru',
        providers: [{ kind: 'provider', id: 'cms', protocol: 'http', baseUrl: 'https://c.example.com' }],
        operations: [
          { kind: 'operation', id: 'content.get', typeOp: 'query', providerId: 'cms', path: '/content/{id}', cache: 'immutable', type: 'content' },
        ],
        endpoints: [], routes: [], themes: [],
      }),
    );
    const op = parsed.operations[0];
    expect(op.id).toBe('content.get');
    expect(op.type).toBe('content');
    expect(op.params).toBeUndefined();
  });
});