import { useCallback, useEffect, useState } from 'react';
import { HexColorPicker, HexColorInput } from 'react-colorful';
import { Inline, Stack } from '@liapoldus/ui-kit';
import type { ColorMode } from '../../runtime';
import { useAdmin } from '../admin-context';

const HEX_RE = /^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$/;

/** Нормализует ввод в валидный hex (#rgb/#rrggbb) или возвращает null. */
export function normalizeHex(input: string): string | null {
  const trimmed = input.trim().replace(/^#/, '');
  if (HEX_RE.test(`#${trimmed}`)) {
    if (trimmed.length === 3) {
      return `#${trimmed
        .split('')
        .map((c) => c + c)
        .join('')}`;
    }
    return `#${trimmed}`;
  }
  return null;
}

export interface ColorPickerProps {
  value: string;
  onChange: (value: string) => void;
  invalid?: boolean;
  /** Подпись рядом с пикером (e.g. "Light"/"Dark"). */
  label?: string;
}

/**
 * Цветопикер для одного цветового токена: палитра react-colorful + hex-поле.
 * Ввод хекса локально буферизуется и валидируется («невалидный цвет» показан
 * под полем), подтверждается в onChange только при валидном значении.
 */
export function ColorPicker({ value, onChange, invalid, label }: ColorPickerProps) {
  const { t } = useAdmin();
  const [draft, setDraft] = useState(value);

  useEffect(() => {
    setDraft(value);
  }, [value]);

  const commit = useCallback(
    (raw: string) => {
      const next = normalizeHex(raw);
      setDraft(raw);
      if (next) onChange(next);
    },
    [onChange],
  );

  const valid = value === '' || HEX_RE.test(value);

  return (
    <Stack gap={2}>
      <HexColorPicker color={value.startsWith('#') ? value : '#000000'} onChange={commit} />
      <Inline gap={2} align="center">
        {label && <span className="text-xs font-medium text-neutral-500">{label}</span>}
        <HexColorInput
          prefixed
          color={draft}
          onChange={(raw) => commit(raw)}
          aria-label={label ?? t('tokens.color.picker')}
          className="h-9 w-32 rounded-md border border-input bg-transparent px-2 text-sm font-mono"
        />
        {!valid && <span className="text-xs text-destructive">{t('tokens.invalidColor')}</span>}
      </Inline>
    </Stack>
  );
}
