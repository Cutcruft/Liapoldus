import { transform } from 'sucrase';

/** Результат live-валидации TSX (sucrase, лёгкий транслер — типы не проверяются). */
export interface TsxDiagnostic {
  ok: boolean;
  line?: number;
  message?: string;
}

/** Отбрасывает суффикс позиции `(N:M)` из сообщения sucrase. */
function cleanMessage(message: string): string {
  return message.replace(/\s*\(\d+:\d+\)\s*$/, '').trim();
}

/**
 * Проверяет TSX-исходник компонента без типизации (решение R5, риск 2 в
 * backend.md: лёгкий транслер в UI, типы — на бэкенде). Пустой/пробельный
 * исходник считается валидным.
 */
export function validateTsx(source: string): TsxDiagnostic {
  if (!source.trim()) return { ok: true };
  try {
    transform(source, { transforms: ['typescript', 'jsx'] });
    return { ok: true };
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err);
    const match = /\((\d+):(\d+)\)$/.exec(message.trim());
    return {
      ok: false,
      line: match ? Number(match[1]) : undefined,
      message: cleanMessage(message) || 'Синтаксическая ошибка',
    };
  }
}