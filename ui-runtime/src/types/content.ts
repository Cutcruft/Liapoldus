/** Контент: значение по ключу, либо многоязычная запись. */
export type ContentRecordValue =
  | string
  | number
  | boolean
  | null
  | ContentRecord[]
  | { [key: string]: ContentRecordValue };

export interface ContentRecord {
  [key: string]: ContentRecordValue | ContentRecord;
}

export type ContentData = Record<string, unknown>;

export interface StringsCollection {
  [key: string]: string;
}

export interface LocalizedContent {
  locale: string;
  data: ContentData;
}

export type LocaleLike = string | Record<string, ContentData> | Record<'locale' | 'payload', unknown>;