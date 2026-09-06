import type { ComponentType } from 'react';
import { ComponentNotFoundError, DuplicateRegistrationError } from '../errors';

/** Компонент как тип: знает только как рендерить (§12a). */
export type ComponentDefinition = ComponentType<Record<string, unknown>>;

/**
 * Реестр определений компонентов (§12a): site-бандл регистрирует свои
 * определения до boot(), page-дерево ссылается на definitionId, PageRenderer
 * забирает React-компонент отсюда и передаёт props + резолвленные binding-значения.
 * Metadata/schema — отдельные дескрипторы, в рантайм не зашиты.
 *
 * Общерпроцессный singleton (статические методы): несколько страниц в одном
 * бандле делят один реестр; при необходимости отдельные экземпляры не создаются —
 * реестр только один, как и runtime-процесс сайта.
 */
export class ComponentRegistry {
  private static definitions = new Map<string, ComponentDefinition>();

  static registerDefinition(id: string, component: ComponentDefinition): void {
    if (this.definitions.has(id)) {
      throw new DuplicateRegistrationError(`Компонент '${id}' уже зарегистрирован`);
    }
    this.definitions.set(id, component);
  }

  static getDefinition(id: string): ComponentDefinition {
    const component = this.definitions.get(id);
    if (!component) {
      throw new ComponentNotFoundError(`Компонент '${id}' не зарегистрирован`);
    }
    return component;
  }

  static hasDefinition(id: string): boolean {
    return this.definitions.has(id);
  }

  /** Зарегистрированные id в порядке регистрации (для построения ComponentMap). */
  static ids(): string[] {
    return [...this.definitions.keys()];
  }

  /** Очистка реестра (тесты, hard-reload). */
  static clear(): void {
    this.definitions.clear();
  }
}