import { useSearchParams } from 'react-router-dom';
import { FormsList } from './FormsList';
import { FormDetail } from './FormDetail';

/**
 * Раздел «Обслуживание → Формы+ответы» (R4). Маршрутизация по URL:
 * `?mode=forms` — список форм, `?mode=forms&formId=...` — ответы выбранной формы.
 */
export function FormsSection() {
  const [searchParams] = useSearchParams();
  const formId = searchParams.get('formId');
  if (formId) {
    return <FormDetail key={formId} formId={formId} />;
  }
  return <FormsList />;
}