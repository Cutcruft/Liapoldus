package domain

import "time"

type Content struct {
	ID           string                    `json:"id"`
	SiteID       string                    `json:"siteId"`
	Key          string                    `json:"-"`
	CollectionID string                    `json:"collectionId"`
	Fields       map[string]any            `json:"fields"`
	Translations map[string]map[string]any `json:"translations"`
	CreatedAt    time.Time                 `json:"createdAt"`
	UpdatedAt    time.Time                 `json:"updatedAt"`
}

// ContentData is the client-facing merged view (base + overlay for a locale).
type ContentData struct {
	ID           string         `json:"id"`
	SiteID       string         `json:"siteId"`
	CollectionID string         `json:"collectionId"`
	Locale       string         `json:"locale"`
	Fields       map[string]any `json:"fields"`
}

// LocaleStat — сколько единиц контента имеют перевод на локаль.
type LocaleStat struct {
	Locale       string `json:"locale"`
	ContentCount int    `json:"contentCount"`
}

// SiteLocales — список локалей сайта и сводка переведённости (R3).
// Locales не включает базовый язык: его содержание — весь контент сайта.
type SiteLocales struct {
	BaseLocale string       `json:"baseLocale"`
	Locales    []LocaleStat `json:"locales"`
	Total      int          `json:"total"`
}
