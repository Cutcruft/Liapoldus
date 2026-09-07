package domain

// ColorMode is one named color mode a color token can carry a value for.
// Sites commonly maintain light and dark values for the same color role.
type ColorMode string

const (
	ColorModeLight ColorMode = "light"
	ColorModeDark  ColorMode = "dark"
)

// ColorToken is one named color role with a value per mode. The palette is
// fully free-form: there are no predefined roles, the editor adds and removes
// them as the site's design language evolves. Values are CSS colors
// ("#0b2e4f", "rgb(12 34 56 / 0.8)", "var(--accent)").
type ColorToken struct {
	Name  string               `json:"name"`
	Value map[ColorMode]string `json:"value"`
}

// FontToken registers one web font: the CSS font-family the face implements,
// the site asset carrying the font file (TTF/WOFF2), and the weight/style the
// file encodes. Materialization turns every font token into an @font-face rule
// in the page head (build layer) and the runtime contract exposes it under
// theme tokens "fonts".
type FontToken struct {
	Family  string `json:"family"`
	AssetID string `json:"assetId"`
	Weight  string `json:"weight,omitempty"`
	Style   string `json:"style,omitempty"`
}

// TokenSet is the complete editable design-token state of a site: the free-form
// color palette plus the categorized scalar groups. Each scalar group maps a
// token name to a CSS declaration value ("2rem", "0 2px 4px rgba(0,0,0,.1)",
// "600ms ease"). Fonts list the registered web-font faces (fonts are assets).
type TokenSet struct {
	Colors      []ColorToken      `json:"colors"`
	Fonts       []FontToken       `json:"fonts"`
	Typography  map[string]string `json:"typography,omitempty"`
	Spacing     map[string]string `json:"spacing,omitempty"`
	Shadows     map[string]string `json:"shadows,omitempty"`
	Borders     map[string]string `json:"borders,omitempty"`
	Breakpoints map[string]string `json:"breakpoints,omitempty"`
	ZIndex      map[string]string `json:"zIndex,omitempty"`
	Opacity     map[string]string `json:"opacity,omitempty"`
	Transitions map[string]string `json:"transitions,omitempty"`
	Custom      map[string]string `json:"custom,omitempty"`
}
