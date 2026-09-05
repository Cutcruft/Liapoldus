package id

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// Prefixes are part of the public API contract (tests and docs assert the
// `<entity>_<hex>` shape), so they are constants rather than configuration.
const (
	Site       = "site"
	Page       = "page"
	PageVer    = "pagever"
	Snapshot   = "snapshot"
	Content    = "content"
	Asset      = "asset"
	Route      = "route"
	Form       = "form"
	Submission = "submission"
)

func New(prefix string) (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	return prefix + "_" + hex.EncodeToString(b), nil
}
