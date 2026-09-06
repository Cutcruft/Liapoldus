import { useState } from 'react';
import { useAdmin } from '../admin-context';

/**
 * Двухшаговое подтверждение деструктивного действия: клик «Удалить» →
 * «Подтвердить»/«Отмена». Избегает window.confirm (нет в jsdom).
 */
export function ConfirmButton({
  label,
  confirmLabel,
  cancelLabel,
  onConfirm,
}: {
  label: string;
  confirmLabel?: string;
  cancelLabel?: string;
  onConfirm: () => void | Promise<void>;
}) {
  const { t } = useAdmin();
  const [armed, setArmed] = useState(false);
  const [busy, setBusy] = useState(false);

  const confirm = () => {
    setBusy(true);
    Promise.resolve(onConfirm()).finally(() => {
      setBusy(false);
      setArmed(false);
    });
  };

  if (armed) {
    return (
      <span className="inline-flex items-center gap-1.5">
        <button
          type="button"
          disabled={busy}
          onClick={confirm}
          className="rounded border border-red-300 bg-red-600 px-2 py-1 text-xs text-white disabled:opacity-50"
        >
          {confirmLabel ?? t('common.confirm')}
        </button>
        <button
          type="button"
          disabled={busy}
          onClick={() => setArmed(false)}
          className="rounded border border-neutral-300 px-2 py-1 text-xs text-neutral-600"
        >
          {cancelLabel ?? t('common.cancel')}
        </button>
      </span>
    );
  }

  return (
    <button
      type="button"
      onClick={() => setArmed(true)}
      className="rounded border border-neutral-300 px-2 py-1 text-xs text-neutral-600 hover:border-red-300 hover:text-red-600"
    >
      {label}
    </button>
  );
}