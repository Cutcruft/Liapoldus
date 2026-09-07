import { useSearchParams } from 'react-router-dom';
import { ContentList } from './ContentList';
import { ContentEditor } from './ContentEditor';

/**
 * Раздел «Обслуживание → Контент» (R3). Маршрутизация по URL:
 * `?mode=content` — список, `?mode=content&contentId=...` — редактор.
 */
export function ContentSection() {
  const [searchParams] = useSearchParams();
  const contentId = searchParams.get('contentId');
  if (contentId) {
    return <ContentEditor key={contentId} contentId={contentId} />;
  }
  return <ContentList />;
}