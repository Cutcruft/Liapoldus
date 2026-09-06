import { useMemo } from 'react';
import { Stack } from '@liapoldus/ui-kit';
import type { TokenSet } from '../../runtime';
import { useAdmin } from '../admin-context';

/** Собирает сниппет `:root` с CSS-переменными --lia-* из набора токенов. */
export function tokensToCss(tokens: TokenSet): string {
  const lines: string[] = [];
  for (const c of tokens.colors) {
    const name = c.name.trim();
    if (!name) continue;
    for (const [mode, v] of Object.entries(c.value)) {
      if (!v) continue;
      lines.push(`  --lia-${name}-${mode}: ${v};`);
    }
  }
  const scalarGroups = [
    'typography',
    'spacing',
    'shadows',
    'borders',
    'breakpoints',
    'zIndex',
    'opacity',
    'transitions',
    'custom',
  ] as const;
  for (const group of scalarGroups) {
    const map = tokens[group];
    if (!map) continue;
    for (const [key, v] of Object.entries(map)) {
      if (!key.trim() || !v) continue;
      lines.push(`  --lia-${group}-${key}: ${v};`);
    }
  }
  return `:root {\n${lines.join('\n')}\n}`;
}

/** Предпросмотр живого CSS-верста как iframe srcdoc (обновляется на каждый кейсток). */
export function TokenPreview({ tokens }: { tokens: TokenSet }) {
  const { t } = useAdmin();
  const css = useMemo(() => tokensToCss(tokens), [tokens]);

  const html = useMemo(
    () =>
      `<!doctype html><html><head><meta charset="utf-8"/><style>
        ${css}
        body{margin:0;padding:16px;font-family:system-ui,sans-serif}
        .swatch{display:flex;align-items:center;gap:8px;margin-bottom:8px}
        .chip{width:36px;height:36px;border-radius:8px;border:1px solid rgba(0,0,0,.15)}
        .row{margin-bottom:4px;font-family:ui-monospace,monospace}
        .k{color:#666;font-size:11px;text-transform:uppercase;margin:12px 0 6px}
      </style></head><body>
        <div class="k">${t('tokens.colors')}</div>
        ${tokens.colors
          .map(
            (c) =>
              `<div class="swatch">
                <span class="chip" style="background:${c.value.light || 'transparent'}"></span>
                <span class="row">--lia-${c.name}-light <code>${c.value.light ?? ''}</code></span>
              </div>
              <div class="swatch">
                <span class="chip" style="background:${c.value.dark || 'transparent'}"></span>
                <span class="row">--lia-${c.name}-dark <code>${c.value.dark ?? ''}</code></span>
              </div>`,
          )
          .join('')}
        ${tokens.colors.length === 0 ? `<div class="k">${t('tokens.colors.none')}</div>` : ''}
        <div class="k">${t('tokens.groups')}</div>
        <div class="row">--lia-spacing-pad: <code>var(--lia-spacing-pad, ${tokens.spacing?.pad ?? ''})</code></div>
      </body></html>`,
    [tokens, t, css],
  );

  return (
    <Stack gap={2}>
      <p className="text-xs text-neutral-400">{t('tokens.preview.hint')}</p>
      <iframe
        title="Token preview"
        srcDoc={html}
        sandbox=""
        className="h-72 w-full rounded-lg border border-neutral-200 bg-white"
      />
    </Stack>
  );
}
