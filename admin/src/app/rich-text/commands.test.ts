import { describe, expect, it } from 'vitest';
import { matchSlashCommands, SLASH_COMMANDS, labelKeyFor, type SlashCommandKind } from './commands';
import { makeTranslate, STRINGS } from '../../runtime';

const labelOf = (kind: SlashCommandKind) => makeTranslate(STRINGS)(labelKeyFor(kind));

describe('commands', () => {
  it('пустой запрос → весь базовый набор', () => {
    expect(matchSlashCommands('', labelOf)).toEqual(SLASH_COMMANDS);
  });

  it('фильтрация по подстроке (ru и латиница)', () => {
    expect(matchSlashCommands('список', labelOf)).toEqual(['bulletList', 'orderedList']);
    expect(matchSlashCommands('h1', labelOf)).toEqual(['h1']);
    expect(matchSlashCommands('изображение', labelOf)).toEqual(['image']);
  });

  it('labelKeyFor покрывает все виды команд', () => {
    for (const kind of SLASH_COMMANDS) {
      expect(STRINGS[labelKeyFor(kind)]).toBeTruthy();
    }
  });
});