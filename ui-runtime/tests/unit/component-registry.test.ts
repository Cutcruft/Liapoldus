import { beforeEach, describe, expect, it } from 'vitest';
import { ComponentNotFoundError, DuplicateRegistrationError } from '../../src/errors';
import { ComponentRegistry } from '../../src/core/component-registry';
import type { ComponentDefinition } from '../../src/core/component-registry';

function DummyComponent() {
  return null;
}

describe('ComponentRegistry (§12a)', () => {
  beforeEach(() => {
    ComponentRegistry.clear();
  });

  it('регистрирует и возвращает определение по id', () => {
    ComponentRegistry.registerDefinition('hero', DummyComponent);
    expect(ComponentRegistry.hasDefinition('hero')).toBe(true);
    expect(ComponentRegistry.getDefinition('hero')).toBe(DummyComponent);
    expect(ComponentRegistry.ids()).toEqual(['hero']);
  });

  it('бросает DuplicateRegistrationError при повторной регистрации', () => {
    ComponentRegistry.registerDefinition('hero', DummyComponent);
    expect(() => ComponentRegistry.registerDefinition('hero', DummyComponent)).toThrow(
      DuplicateRegistrationError,
    );
  });

  it('бросает ComponentNotFoundError для неизвестного id', () => {
    expect(ComponentRegistry.hasDefinition('missing')).toBe(false);
    expect(() => ComponentRegistry.getDefinition('missing')).toThrow(ComponentNotFoundError);
  });

  it('ids() возвращает порядок регистрации', () => {
    ComponentRegistry.registerDefinition('a', DummyComponent);
    ComponentRegistry.registerDefinition('b', DummyComponent);
    expect(ComponentRegistry.ids()).toEqual(['a', 'b']);
  });

  it('ComponentDefinition типизирован как React-компонент', () => {
    const definition: ComponentDefinition = DummyComponent;
    expect(typeof definition).toBe('function');
  });
});