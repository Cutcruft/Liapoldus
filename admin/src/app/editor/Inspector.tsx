import { lazy, Suspense } from 'react';
import { useSelector } from '@liapoldus/ui-runtime';
import type { JSONSchema } from './schemas';
import type { BindingSourceName } from './field-form';
import { SchemaForm, type ValidationCode } from './field-form';
import { findSelected } from './TreePanel';
import { AssetFieldControl } from '../assets/AssetFieldControl';
import type { EditorStore, EditorActions } from './page-store';
import type { AdminStringKey, Translate } from '../../runtime';

// Tiptap/ProseMirror — отдельный chunk, грузится только при инспекции текстового узла.
const RichTextEditor = lazy(() =>
  import('../rich-text/RichTextEditor').then((m) => ({ default: m.RichTextEditor })),
);

const BINDING_LABEL: Record<BindingSourceName, AdminStringKey> = {
  literal: 'editor.binding.literal',
  content: 'editor.binding.content',
  form: 'editor.binding.form',
  operation: 'editor.binding.operation',
  query: 'editor.binding.query',
  routeGroup: 'editor.binding.routeGroup',
};

const VALIDATION_LABEL: Record<ValidationCode, AdminStringKey> = {
  required: 'common.required',
  type: 'field.error.type',
  enum: 'field.error.enum',
  range: 'field.error.range',
};

export function Inspector({
  store,
  actions,
  t,
  siteId,
  schemasByType,
}: {
  store: EditorStore;
  actions: EditorActions;
  t: Translate;
  siteId: string;
  schemasByType: Record<string, JSONSchema>;
}) {
  const elements = useSelector(store, (s) => s.elements);
  const selectionId = useSelector(store, (s) => s.selectionId);
  const el = findSelected(elements, selectionId);
  const schema: JSONSchema | undefined = el ? schemasByType[el.componentId] : undefined;

  const sourceLabel = (s: BindingSourceName) => t(BINDING_LABEL[s]);
  const errorLabel = (code: ValidationCode) => t(VALIDATION_LABEL[code]);

  return (
    <div className="flex flex-col gap-3">
      <h2 className="text-sm font-medium uppercase tracking-wide text-neutral-400">
        {t('editor.props')} {el ? `· ${el.componentId}` : ''}
      </h2>

      {!el && <p className="text-xs text-neutral-400">{t('editor.selectHint')}</p>}

      {el && !schema && (
        <p className="text-xs text-neutral-400">{t('editor.unknownType', { type: el.componentId })}</p>
      )}

      {el && schema && (
        <SchemaForm
          schema={schema}
          props={el.props}
          sourceLabel={sourceLabel}
          pathLabel={t('editor.binding.path')}
          errorLabel={errorLabel}
          onChange={(field, value) => actions.updateProp(el.id, field, value)}
          onBindingChange={(field, source) => actions.setBinding(el.id, field, source)}
          renderAssetField={(value, onChange) => (
            <AssetFieldControl siteId={siteId} value={value} onChange={(id) => onChange(id)} />
          )}
          renderRichTextField={(value, onChange) => (
            <Suspense fallback={<div className="h-24 rounded border border-neutral-200 bg-neutral-50" />}>
              <RichTextEditor siteId={siteId} value={String(value ?? '')} onChange={(html) => onChange(html)} />
            </Suspense>
          )}
        />
      )}
    </div>
  );
}