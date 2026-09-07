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
  validateBindingSource,
  validateElementDescriptor,
  validatePageDescriptor,
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

  it('page: элементы с literal/binding props парсятся; pages наполняется', () => {
    const parsed = parseDescriptors(
      JSON.stringify({
        siteId: 's', environment: 'prod', version: '1', locale: 'ru',
        providers: [], operations: [], endpoints: [], routes: [], themes: [],
        pages: [
          {
            id: 'page.home',
            name: 'Home',
            elements: [
              { id: 'el1', componentId: 'Text', props: { text: { kind: 'literal', value: 'Привет' } } },
              {
                id: 'el2',
                componentId: 'Title',
                props: { title: { kind: 'binding', source: { kind: 'content', contentId: 'hero', field: 'title' } } },
              },
            ],
          },
        ],
      }),
    );
    expect(parsed.pages).toHaveLength(1);
    expect(parsed.pages[0].id).toBe('page.home');
    expect(parsed.pages[0].elements).toHaveLength(2);
    const el2 = parsed.pages[0].elements[1];
    expect(el2.props.title).toEqual({
      kind: 'binding',
      source: { kind: 'content', contentId: 'hero', field: 'title' },
    });
  });

  it('page: без elements или без id → DescriptorValidationError', () => {
    expect(() => validatePageDescriptor({ id: 'p' } as never)).toThrow(DescriptorValidationError);
    expect(() => validatePageDescriptor({ name: 'x' } as never)).toThrow(DescriptorValidationError);
  });

  it('element: props с неизвестным kind → DescriptorValidationError', () => {
    const el = {
      id: 'el1',
      componentId: 'Text',
      props: { text: { kind: 'runtime', path: 'x' } },
    };
    expect(() => validateElementDescriptor(el as never)).toThrow(DescriptorValidationError);
    expect(() => validateElementDescriptor({ id: 'el1' } as never)).toThrow(DescriptorValidationError);
  });

  it('binding source: принятые виды и невалидные → ошибка', () => {
    expect(validateBindingSource({ kind: 'content', contentId: 'c', field: 'x' })).toEqual({
      kind: 'content', contentId: 'c', field: 'x',
    });
    expect(validateBindingSource({ kind: 'routeGroup', index: 2 })).toEqual({ kind: 'routeGroup', index: 2 });
    expect(() => validateBindingSource({ kind: 'routeGroup' } as never)).toThrow(DescriptorValidationError);
    expect(() => validateBindingSource({ kind: 'props' } as never)).toThrow(DescriptorValidationError);
    expect(() => validateBindingSource({ kind: 'content' } as never)).toThrow(DescriptorValidationError);
  });

  it('page: layoutSectionId/head парсятся в PageDescriptor (R10 P1)', () => {
    expect(
      validatePageDescriptor({
        id: 'page.home',
        elements: [],
        layoutSectionId: 'layout.shell',
        head: { title: 'Home', description: 'desc', meta: { robots: 'noindex' } },
      }),
    ).toMatchObject({
      layoutSectionId: 'layout.shell',
      head: { title: 'Home', description: 'desc', meta: { robots: 'noindex' } },
    });
  });

  it('contract: site head/defaultLayoutSectionId парсятся (R10 P1)', () => {
    const parsed = parseDescriptors(
      JSON.stringify({
        siteId: 's', environment: 'prod', version: '1', locale: 'ru',
        providers: [], operations: [], endpoints: [], routes: [], themes: [],
        head: { titleTemplate: '{title} — ACME', description: 'site', og: { type: 'website' } },
        defaultLayoutSectionId: 'layout.shell',
      }),
    );
    expect(parsed.contract.defaultLayoutSectionId).toBe('layout.shell');
    expect(parsed.contract.head).toMatchObject({
      titleTemplate: '{title} — ACME',
      description: 'site',
      og: { type: 'website' },
    });
  });
});