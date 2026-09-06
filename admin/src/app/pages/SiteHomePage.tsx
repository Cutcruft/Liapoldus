import { useState, type FormEvent } from 'react';
import { NavLink, useNavigate, useParams } from 'react-router-dom';
import { Inline, Stack } from '@liapoldus/ui-kit';
import { runOperation, type Site } from '../../runtime';
import { useAdmin } from '../admin-context';
import { useOperation } from '../use-operation';
import { ConfirmButton } from '../components/ConfirmButton';
import { Field } from '../components/Field';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';

const SELECT_CLASS =
  'h-9 rounded-md border border-input bg-transparent px-3 text-sm';

const SECTIONS: Array<{
  to: string;
  labelKey: 'nav.pages' | 'nav.contents' | 'nav.assets' | 'nav.routes' | 'nav.forms' | 'nav.builds' | 'nav.git' | 'nav.tokens';
}> = [
  { to: 'pages', labelKey: 'nav.pages' },
  { to: 'contents', labelKey: 'nav.contents' },
  { to: 'assets', labelKey: 'nav.assets' },
  { to: 'routes', labelKey: 'nav.routes' },
  { to: 'forms', labelKey: 'nav.forms' },
  { to: 'builds', labelKey: 'nav.builds' },
  { to: 'git', labelKey: 'nav.git' },
  { to: 'tokens', labelKey: 'nav.tokens' },
];

export function SiteHomePage() {
  const { siteId = '' } = useParams();
  const { api, t } = useAdmin();
  const navigate = useNavigate();
  const site = useOperation<Site | null>('getSite', { siteId }, (d: unknown) =>
    typeof d === 'object' && d !== null && 'id' in d ? (d as Site) : null,
  );

  const remove = async () => {
    const res = await runOperation(api, 'deleteSite', { siteId }, t);
    if (res.ok) navigate('/sites');
  };

  const [editing, setEditing] = useState(false);
  const [name, setName] = useState('');
  const [locale, setLocale] = useState('ru');
  const [hosts, setHosts] = useState<string[]>(['']);
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState('');

  const startEdit = (site: Site) => {
    setName(site.name);
    setLocale(site.defaultLocale);
    setHosts(site.hosts.length ? [...site.hosts] : ['']);
    setSaveError('');
    setEditing(true);
  };

  const save = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    if (!name.trim()) {
      setSaveError(t('common.required'));
      return;
    }
    const cleaned = hosts.map((h) => h.trim()).filter(Boolean);
    setSaving(true);
    setSaveError('');
    const res = await runOperation(
      api,
      'updateSite',
      { siteId, patch: { name: name.trim(), defaultLocale: locale, hosts: cleaned } },
      t,
    );
    setSaving(false);
    if (!res.ok) {
      setSaveError(res.detail);
      return;
    }
    setEditing(false);
    site.reload();
  };

  const changeHost = (i: number, value: string) =>
    setHosts((hs) => hs.map((h, j) => (j === i ? value : h)));

  const current = site.state.status === 'success' ? site.state.data : null;

  return (
    <Stack pad={8} gap={4}>
      {site.state.status === 'loading' && <p className="text-sm text-neutral-400">{t('common.loading')}</p>}

      {site.state.status === 'error' && (
        <Stack gap={2}>
          <p className="text-sm text-red-600">
            {t('common.error')}: {site.state.detail}
          </p>
          <button type="button" onClick={site.reload} className="w-fit rounded border border-neutral-300 px-3 py-1 text-sm">
            {t('common.reload')}
          </button>
        </Stack>
      )}

      {site.state.status === 'success' &&
        (current ? (
          <>
            <Inline justify="between" align="center">
              <div>
                <h1 className="text-xl font-medium">{current.name}</h1>
                <p className="text-sm text-neutral-500">
                  <code>{current.slug}</code> · локали: {current.defaultLocale} · хосты:{' '}
                  {current.hosts.join(', ') || '—'}
                </p>
              </div>
              <Inline gap={2}>
                <Button type="button" variant="outline" onClick={() => startEdit(current!)}>
                  {t('site.edit')}
                </Button>
                <ConfirmButton label={t('site.delete.confirm')} onConfirm={remove} />
              </Inline>
            </Inline>

            {editing && (
              <form onSubmit={(e) => void save(e)} className="rounded-lg border border-neutral-200 p-4">
                <Stack gap={3}>
                  <Inline gap={3} align="end">
                    <Field label={t('site.name')} required>
                      <Input value={name} onChange={(e) => setName(e.target.value)} />
                    </Field>
                    <Field label={t('site.slug')}>
                      <Input value={current.slug} disabled />
                    </Field>
                    <Field label={t('site.locale')}>
                      <select className={SELECT_CLASS} value={locale} onChange={(e) => setLocale(e.target.value)}>
                        <option value="ru">ru</option>
                        <option value="en">en</option>
                      </select>
                    </Field>
                  </Inline>
                  <p className="text-xs text-neutral-400">{t('site.edit.slugNote')}</p>

                  <Field label={t('site.edit.hosts')}>
                    <Stack gap={2}>
                      {hosts.map((h, i) => (
                        <Inline key={i} gap={2}>
                          <Input
                            value={h}
                            placeholder={t('site.edit.hostPlaceholder')}
                            onChange={(e) => changeHost(i, e.target.value)}
                          />
                          <Button
                            type="button"
                            variant="outline"
                            size="sm"
                            onClick={() => setHosts((hs) => hs.filter((_, j) => j !== i))}
                          >
                            ×
                          </Button>
                        </Inline>
                      ))}
                    </Stack>
                  </Field>
                  <Inline gap={2}>
                    <Button type="button" variant="outline" size="sm" onClick={() => setHosts((hs) => [...hs, ''])}>
                      {t('site.edit.addHost')}
                    </Button>
                  </Inline>

                  <Inline gap={2}>
                    <Button type="submit" disabled={saving}>
                      {t('site.edit.save')}
                    </Button>
                    <Button type="button" variant="outline" onClick={() => setEditing(false)}>
                      {t('common.cancel')}
                    </Button>
                  </Inline>
                  {saveError && <p className="text-sm text-destructive">{saveError}</p>}
                </Stack>
              </form>
            )}

            <nav aria-label="Разделы сайта">
              <Stack gap={2}>
                <h2 className="text-sm font-medium text-neutral-500">{t('site.sections')}</h2>
                <Inline gap={2}>
                  {SECTIONS.map((s) => (
                    <NavLink
                      key={s.to}
                      to={s.to}
                      className={({ isActive }) =>
                        `rounded border px-3 py-1.5 text-sm ${
                          isActive ? 'border-blue-300 bg-blue-50 text-blue-700' : 'border-neutral-300 text-neutral-600'
                        }`
                      }
                    >
                      {t(s.labelKey)}
                    </NavLink>
                  ))}
                </Inline>
              </Stack>
            </nav>
          </>
        ) : (
          <p className="text-sm text-neutral-400">{t('site.notFound')}</p>
        ))}
    </Stack>
  );
}