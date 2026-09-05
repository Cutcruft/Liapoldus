export interface AssetVariant {
  name: string;
  url: string;
}

export interface AssetMeta {
  id: string;
  name: string;
  type: string;
  size: number;
  variants: AssetVariant[];
}

export interface AssetPayload {
  /** id ассета или специальное значение `{ref}` */
  asset: string | { ref: string };
  /** dot-path к вложенному значению */
  path?: string;
}

export interface AssetRefDirective {
  ref: string;
  path?: string;
}