import { useAdmin } from '../admin-context';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog';
import { Button } from '@/components/ui/button';

/**
 * Подтверждение деструктивного действия через AlertDialog. Избегает window.confirm.
 */
export function ConfirmButton({
  label,
  confirmLabel,
  cancelLabel,
  description,
  onConfirm,
}: {
  label: string;
  confirmLabel?: string;
  cancelLabel?: string;
  description?: string;
  onConfirm: () => void | Promise<void>;
}) {
  const { t } = useAdmin();

  return (
    <AlertDialog>
      <AlertDialogTrigger asChild>
        <Button type="button" variant="outline" size="sm">
          {label}
        </Button>
      </AlertDialogTrigger>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{label}</AlertDialogTitle>
          {description && <AlertDialogDescription>{description}</AlertDialogDescription>}
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel asChild>
            <Button type="button" variant="outline" size="sm">
              {cancelLabel ?? t('common.cancel')}
            </Button>
          </AlertDialogCancel>
          <AlertDialogAction asChild>
            <Button
              type="button"
              variant="destructive"
              size="sm"
              onClick={() => {
                void onConfirm();
              }}
            >
              {confirmLabel ?? t('common.confirm')}
            </Button>
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}