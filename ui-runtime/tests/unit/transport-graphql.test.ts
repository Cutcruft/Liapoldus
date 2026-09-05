import { describe, it, expect, beforeEach } from 'vitest';
import { GraphqlTransport } from '../../src/core/transport/graphql';
import { RuntimeRegistry } from '../../src/core/registry';
import { TransportError } from '../../src/errors';
import { providerDescriptor4, operationDescriptor8 } from './fixtures';
import { makeFakeFetch } from './helpers';

describe('GraphqlTransport', () => {
  let registry: RuntimeRegistry;

  beforeEach(() => {
    registry = new RuntimeRegistry();
    registry.register(providerDescriptor4);
    registry.register(operationDescriptor8);
  });

  it('POST {query, variables} на baseUrl provider', async () => {
    const { fetch, calls } = makeFakeFetch(() => ({ viewer: { id: 'u1' } }));
    const transport = new GraphqlTransport(registry.getProvider('gql')!, { fetch });
    const provider = registry.getProvider('gql')!;
    const res = await transport.request({
      provider,
      operation: registry.getOperation('gql.query')!,
      gql: '{ viewer { id } }',
      input: { deviceId: 'd1' },
    });
    expect(calls[0].url).toBe('https://graphql.example.com/graphql');
    expect(calls[0].init.method).toBe('POST');
    const body = JSON.parse(calls[0].init.body as string);
    expect(body.query).toBe('{ viewer { id } }');
    expect(body.variables).toEqual({ deviceId: 'd1' });
    expect(res.body).toEqual({ viewer: { id: 'u1' } });
  });

  it('GraphQL-ошибки → TransportError с graphqlErrors', async () => {
    const { fetch } = makeFakeFetch(() => ({ data: null, errors: [{ message: 'DENIED' }] }));
    const transport = new GraphqlTransport(registry.getProvider('gql')!, { fetch });
    const provider = registry.getProvider('gql')!;
    const promise = transport.request({ provider, operation: registry.getOperation('gql.query')!, gql: '{ viewer { id } }' });
    await expect(promise).rejects.toMatchObject({
      code: 'transport_error',
      cause: { graphqlErrors: [{ message: 'DENIED' }] },
    });
  });

  it('без gql → TransportError', async () => {
    const transport = new GraphqlTransport(registry.getProvider('gql')!, { fetch: makeFakeFetch(() => null).fetch });
    const provider = registry.getProvider('gql')!;
    await expect(transport.request({ provider, operation: registry.getOperation('gql.query')! })).rejects.toBeInstanceOf(TransportError);
  });
});