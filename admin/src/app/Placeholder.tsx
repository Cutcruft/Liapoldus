import { Stack } from '@liapoldus/ui-kit';
import { useAdmin } from './admin-context';
import type { AdminStringKey } from '../runtime';

export function Placeholder({ titleKey, scope }: { titleKey: AdminStringKey; scope?: string }) {
  const { t } = useAdmin();
  return (
    <Stack pad={8} gap={4}>
      <h1 className="text-xl font-medium">{t(titleKey)}</h1>
      {scope && <p className="text-sm text-neutral-500">Сайт: {scope}</p>}
      <p className="text-sm text-neutral-400">M1: наполнение данными через AdminApi.</p>
    </Stack>
  );
}