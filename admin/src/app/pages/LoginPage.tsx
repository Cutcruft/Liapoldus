import { useState, type FormEvent } from 'react';
import { useNavigate } from 'react-router-dom';
import { Stack } from '@liapoldus/ui-kit';
import { validateToken } from '../../runtime';
import { useAdmin, ADMIN_TOKEN_KEY } from '../admin-context';
import { Field } from '../components/Field';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';

/**
 * Страница входа (слайс 2.4): проверяет токен на сервере, пишет его в
 * tokenStore и localStorage, затем редиректит на дашборд. Снаружи AppShell.
 */
export function LoginPage() {
  const { api, tokenStore, t } = useAdmin();
  const navigate = useNavigate();
  const [token, setToken] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  const submit = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    if (busy) return;
    const value = token.trim();
    if (!value) {
      setError(t('auth.login.empty'));
      return;
    }
    setBusy(true);
    setError('');
    const valid = await validateToken(api, value);
    setBusy(false);
    if (!valid) {
      setError(t('auth.login.invalid'));
      return;
    }
    tokenStore.setState({ token: value });
    globalThis.localStorage?.setItem(ADMIN_TOKEN_KEY, value);
    navigate('/', { replace: true });
  };

  return (
    <div className="flex min-h-full items-center justify-center bg-neutral-50 p-4">
      <form
        onSubmit={(e) => void submit(e)}
        className="w-full max-w-sm rounded-lg border border-neutral-200 bg-white p-6 shadow-sm"
      >
        <Stack gap={4}>
          <div>
            <h1 className="text-lg font-semibold text-neutral-900">Liapoldus</h1>
            <p className="text-sm text-neutral-500">{t('auth.login.title')}</p>
          </div>

          <Field label={t('auth.login.token')}>
            <Input
              type="password"
              value={token}
              onChange={(e) => setToken(e.target.value)}
              placeholder="••••••••"
              aria-label={t('auth.login.token')}
              autoFocus
            />
          </Field>

          {error && <p className="text-sm text-red-600">{error}</p>}

          <Button type="submit" disabled={busy}>
            {t('auth.login.submit')}
          </Button>

          <p className="text-xs text-neutral-400">{t('auth.login.hint')}</p>
        </Stack>
      </form>
    </div>
  );
}