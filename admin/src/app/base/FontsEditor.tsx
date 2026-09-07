import { useState } from 'react';
import { Inline, Stack } from '@liapoldus/ui-kit';
import type { FontToken, Translate } from '../../runtime';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { AssetPicker } from '../assets/AssetPicker';

/**
 * Управляемый редактор веб-шрифтов: каждое семейство — файл TTF/WOFF2 из
 * медиатеки плюс начертание и стиль. Пишет в тот же TokenSet (fonts), что и
 * таб «Токены», поэтому сохранение остаётся одним whole-set PUT (без
 * клобберинга между табами).
 */
export function FontsEditor({
  fonts,
  onChange,
  siteId,
  t,
}: {
  fonts: FontToken[];
  onChange: (next: FontToken[]) => void;
  siteId: string;
  t: Translate;
}) {
  const [pickerRow, setPickerRow] = useState<number | null>(null);

  const setFont = (i: number, patch: Partial<FontToken>) =>
    onChange(fonts.map((f, j) => (j === i ? { ...f, ...patch } : f)));
  const addFont = () => onChange([...fonts, { family: '', assetId: '', weight: '', style: '' }]);
  const removeFont = (i: number) => onChange(fonts.filter((_, j) => j !== i));

  return (
    <Stack gap={3}>
      <section className="rounded-lg border border-neutral-200 p-4">
        <Stack gap={3}>
          <Inline justify="between" align="center">
            <h2 className="text-sm font-medium text-neutral-600">{t('fonts.title')}</h2>
            <Button type="button" variant="outline" size="sm" onClick={addFont}>
              {t('fonts.add')}
            </Button>
          </Inline>
          <p className="text-xs text-neutral-400">{t('fonts.hint')}</p>
          {fonts.length === 0 && <p className="text-sm text-neutral-400">{t('fonts.none')}</p>}
          {fonts.map((f, i) => (
            <div key={i} className="rounded-md border border-neutral-100 p-3">
              <Inline gap={3} align="start">
                <Stack gap={2} className="min-w-[10rem] flex-1">
                  <Inline gap={2}>
                    <Input
                      aria-label={t('fonts.family')}
                      placeholder={t('fonts.family')}
                      value={f.family}
                      onChange={(e) => setFont(i, { family: e.target.value })}
                    />
                    <Input
                      aria-label={t('fonts.weight')}
                      placeholder={t('fonts.weight')}
                      value={f.weight ?? ''}
                      onChange={(e) => setFont(i, { weight: e.target.value })}
                      className="w-24"
                    />
                    <select
                      className="h-9 rounded-md border border-input bg-transparent px-2 text-sm"
                      value={f.style ?? 'normal'}
                      onChange={(e) => setFont(i, { style: e.target.value })}
                      aria-label={t('fonts.style')}
                    >
                      <option value="normal">{t('fonts.style.normal')}</option>
                      <option value="italic">{t('fonts.style.italic')}</option>
                    </select>
                  </Inline>
                  <div className="flex items-center gap-2">
                    <Input
                      readOnly
                      className="flex-1"
                      value={f.assetId}
                      placeholder={t('fonts.file.none')}
                      aria-label={t('fonts.file')}
                    />
                    <Button type="button" variant="outline" size="sm" onClick={() => setPickerRow(i)}>
                      {t('fonts.chooseAsset')}
                    </Button>
                    {f.assetId && (
                      <Button type="button" variant="outline" size="sm" onClick={() => setFont(i, { assetId: '' })}>
                        {t('tokens.remove')}
                      </Button>
                    )}
                  </div>
                </Stack>
                <Button type="button" variant="outline" size="sm" onClick={() => removeFont(i)}>
                  {t('tokens.remove')}
                </Button>
              </Inline>
            </div>
          ))}
        </Stack>
      </section>
      {pickerRow !== null && (
        <AssetPicker
          open
          onClose={() => setPickerRow(null)}
          siteId={siteId}
          selectedId={fonts[pickerRow]?.assetId}
          onSelect={(assetId) => {
            setFont(pickerRow, { assetId });
            setPickerRow(null);
          }}
        />
      )}
    </Stack>
  );
}