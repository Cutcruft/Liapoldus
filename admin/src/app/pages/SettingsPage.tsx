import { useState, type FormEvent } from 'react';
import { Divider, Inline, Stack } from '@liapoldus/ui-kit';
import { APP_VERSION, validateToken, type Settings } from '../../runtime';
import { useAdmin, ADMIN_TOKEN_KEY } from '../admin-context';
import { useOperation } from '../use-operation';
import { Field } from '../components/Field';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Skeleton } from '@/components/ui/skeleton';

const SETTINGS_ROW_CLASS = 'grid grid-cols-[minmax(0,14rem)_minmax(0,1fr)] items-center gap-4 py-2';

/**
 * Страница «Настройки» (слайс 2.6): серверная конфигурация read-only +
 * ротация admin-токена (валидация нового токена и сохранение локально).
 */
export function SettingsPage() {
  const { api, tokenStore, t } = useAdmin();

  const settings = useOperation<Settings>('getSettings', {}, (d) =>
    typeof d === 'object' && d !== null
      ? (d as Settings)
      : { adminToken: '', defaultLocale: '', redirectDefaultStatus: 0 },
  );

  const [rotating, setRotating] = useState(false);
  const [newToken, setNewToken] = useState('');
  const [message, setMessage] = useState<{ ok: boolean; text: string } | null>(null);

  const rotate = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    const value = newToken.trim();
    if (!value) return;
    setRotating(true);
    setMessage(null);
    const valid = await validateToken(api, value);
    setRotating(false);
    if (!valid) {
      setMessage({ ok: false, text: t('settings.rotate.invalid') });
      return;
    }
    tokenStore.setState({ token: value });
    globalThis.localStorage?.setItem(ADMIN_TOKEN_KEY, value);
    setNewToken('');
    setMessage({ ok: true, text: t('settings.rotate.saved') });
  };

  return (
    <Stack pad={8} gap={4} className="max-w-2xl">
      <h1 className="text-xl font-medium">{t('settings.title')}</h1>

      {settings.state.status === 'loading' && <Skeleton className="h-40 w-full" />}

      {settings.state.status === 'error' && (
        <Stack gap={2}>
          <p className="text-sm text-red-600">
            {t('common.error')}: {settings.state.detail}
          </p>
          <Button type="button" variant="outline" onClick={settings.reload} className="w-fit">
            {t('common.reload')}
          </Button>
        </Stack>
      )}

      {settings.state.status === 'success' && (
        <Stack gap={2} className="rounded-lg border border-neutral-200 p-4">
          <div className={SETTINGS_ROW_CLASS}>
            <span className="text-sm font-medium text-neutral-700">{t('settings.adminToken')}</span>
            <code className="text-sm text-neutral-500">{settings.state.data.adminToken || '—'}</code>
          </div>
          <Divider space={1} />
          <div className={SETTINGS_ROW_CLASS}>
            <span className="text-sm font-medium text-neutral-700">{t('settings.defaultLocale')}</span>
            <select
              className="h-9 w-full rounded-md border border-input bg-transparent px-3 text-sm"
              value={settings.state.data.defaultLocale}
              disabled
              aria-label={t('settings.defaultLocale')}
            >
              {['ru', 'en', 'de', 'fr'].map((l) => (
                <option key={l} value={l}>
                  {l}
                </option>
              ))}
            </select>
          </div>
          <Divider space={1} />
          <div className={SETTINGS_ROW_CLASS}>
            <span className="text-sm font-medium text-neutral-700">{t('settings.redirectDefaultStatus')}</span>
            <select
              className="h-9 w-full rounded-md border border-input bg-transparent px-3 text-sm"
              value={String(settings.state.data.redirectDefaultStatus)}
              disabled
              aria-label={t('settings.redirectDefaultStatus')}
            >
              {[301, 302, 307, 308].map((s) => (
                <option key={s} value={String(s)}>
                  {s}
                </option>
              ))}
            </select>
          </div>
          <p className="text-xs text-neutral-400">{t('settings.readonly')}</p>
        </Stack>
      )}

      <form onSubmit={(e) => void rotate(e)} className="rounded-lg border border-neutral-200 p-4">
        <Stack gap={3}>
          <h2 className="text-base font-medium text-neutral-700">{t('settings.rotate')}</h2>
          <Inline gap={3} align="end">
            <Field label={t('settings.rotate')}>
              <Input
                type="password"
                value={newToken}
                onChange={(e) => setNewToken(e.target.value)}
                placeholder="••••••••"
                aria-label={t('settings.rotate')}
              />
            </Field>
            <Button type="submit" disabled={rotating}>
              {t('settings.rotate.submit')}
            </Button>
          </Inline>
          {message && (
            <p className={`text-sm ${message.ok ? 'text-green-600' : 'text-red-600'}`}>{message.text}</p>
          )}
        </Stack>
      </form>

      <Divider space={2} />
      <p className="text-xs text-neutral-400">
        {t('settings.version')}: {APP_VERSION}
      </p>
    </Stack>
  );
}