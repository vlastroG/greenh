package core

// MoisturePct maps a raw sample to 0-100 %. Sensors read higher when drier.
// Callers guarantee dryRaw != wetRaw.
func MoisturePct(raw, dryRaw, wetRaw int) float64 {
	pct := float64(dryRaw-raw) / float64(dryRaw-wetRaw) * 100
	return min(max(pct, 0), 100)
}
