import { useSelector } from '@liapoldus/ui-runtime';
import type { JSONSchema } from './schemas';
import { builtinFieldSchema } from './schemas';
import type { BindingSourceName } from './field-form';
import { SchemaForm, type ValidationCode } from './field-form';
import { findSelected } from './TreePanel';
import { AssetFieldControl } from '../assets/AssetFieldControl';
import type { EditorStore, EditorActions } from './page-store';
import type { AdminStringKey, Translate } from '../../runtime';

const BINDING_LABEL: Record<BindingSourceName, AdminStringKey> = {
  literal: 'editor.binding.literal',
  content: 'editor.binding.content',
  route: 'editor.binding.route',
  query: 'editor.binding.query',
  operation: 'editor.binding.operation',
  form: 'editor.binding.form',
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
}: {
  store: EditorStore;
  actions: EditorActions;
  t: Translate;
  siteId: string;
}) {
  const tree = useSelector(store, (s) => s.tree);
  const selectionId = useSelector(store, (s) => s.selectionId);
  const node = findSelected(tree, selectionId);
  const schema: JSONSchema | undefined = node ? builtinFieldSchema(node.type) : undefined;

  const sourceLabel = (s: BindingSourceName) => t(BINDING_LABEL[s]);
  const errorLabel = (code: ValidationCode) => t(VALIDATION_LABEL[code]);

  return (
    <div className="flex flex-col gap-3">
      <h2 className="text-sm font-medium uppercase tracking-wide text-neutral-400">
        {t('editor.props')} {node ? `· ${node.type}` : ''}
      </h2>

      {!node && <p className="text-xs text-neutral-400">{t('editor.selectHint')}</p>}

      {node && !schema && (
        <p className="text-xs text-neutral-400">{t('editor.unknownType', { type: node.type })}</p>
      )}

      {node && schema && (
        <SchemaForm
          schema={schema}
          values={node.props ?? {}}
          bindings={node.bindings ?? {}}
          sourceLabel={sourceLabel}
          pathLabel={t('editor.binding.path')}
          errorLabel={errorLabel}
          onChange={(field, value) => actions.updateProp(node.id, field, value)}
          onBindingChange={(field, binding) => actions.setBinding(node.id, field, binding)}
          renderAssetField={(value, onChange) => (
            <AssetFieldControl siteId={siteId} value={value} onChange={(id) => onChange(id)} />
          )}
        />
      )}
    </div>
  );
}