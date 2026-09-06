import { useState, type FormEvent } from 'react';
import { useParams } from 'react-router-dom';
import { Inline, Stack } from '@liapoldus/ui-kit';
import { runOperation } from '../../runtime';
import { useAdmin } from '../admin-context';
import { useOperation } from '../use-operation';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Field } from '../components/Field';

export function SiteAllowlistPage() {
  const { siteId = '' } = useParams();
  const { api, t } = useAdmin();

  const entries = useOperation<string[]>('listAllowlist', { siteId }, (d: unknown) =>
    Array.isArray(d) ? (d as string[]) : [],
  );

  const [value, setValue] = useState('');
  const [error, setError] = useState('');
  const [adding, setAdding] = useState(false);

  const add = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    if (!value.trim()) {
      setError(t('common.required'));
      return;
    }
    setAdding(true);
    setError('');
    const res = await runOperation(api, 'addAllowlist', { siteId, entry: value.trim() }, t);
    setAdding(false);
    if (!res.ok) {
      setError(res.detail);
      return;
    }
    setValue('');
    entries.reload();
  };

  const remove = async (entry: string) => {
    const res = await runOperation(api, 'removeAllowlist', { siteId, entry }, t);
    if (!res.ok) {
      setError(res.detail);
      return;
    }
    entries.reload();
  };

  const list = entries.state.status === 'success' ? entries.state.data : [];

  return (
    <Stack gap={6} pad={4}>
      <div>
        <h1 className="text-lg font-medium">{t('allowlist.title')}</h1>
        <p className="text-sm text-neutral-500">{t('allowlist.hint')}</p>
      </div>

      <section className="rounded-lg border border-neutral-200 p-4">
        <Stack gap={4}>
          {entries.state.status === 'loading' && <p className="text-sm text-neutral-400">{t('allowlist.loading')}</p>}
          {entries.state.status === 'error' && (
            <Inline gap={2} align="center">
              <p className="text-sm text-red-600">{t('allowlist.error', { detail: entries.state.detail })}</p>
              <Button type="button" variant="outline" size="sm" onClick={entries.reload}>
                {t('allowlist.reload')}
              </Button>
            </Inline>
          )}
          {entries.state.status === 'success' && (
            <>
              {list.length === 0 && <p className="text-sm text-neutral-400">{t('allowlist.none')}</p>}
              {list.map((entry) => (
                <Inline
                  key={entry}
                  gap={2}
                  justify="between"
                  align="center"
                  className="rounded-md border border-neutral-100 px-3 py-2"
                >
                  <code className="text-sm">{entry}</code>
                  <Button type="button" variant="outline" size="sm" onClick={() => void remove(entry)}>
                    {t('allowlist.remove')}
                  </Button>
                </Inline>
              ))}

              <form onSubmit={(e) => void add(e)} className="mt-2 border-t border-neutral-100 pt-3">
                <Inline gap={2} align="end">
                  <Field label={t('allowlist.entry')}>
                    <Input
                      aria-label={t('allowlist.entry')}
                      placeholder={t('allowlist.entry.placeholder')}
                      value={value}
                      onChange={(e) => setValue(e.target.value)}
                    />
                  </Field>
                  <Button type="submit" disabled={adding}>
                    {t('allowlist.add')}
                  </Button>
                </Inline>
                {error && <p className="mt-2 text-sm text-destructive">{error}</p>}
              </form>
            </>
          )}
        </Stack>
      </section>
    </Stack>
  );
}