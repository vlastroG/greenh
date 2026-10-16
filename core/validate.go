package core

import "fmt"

const maxReasonLen = 200

// ValidateRecommendation checks an advisor answer and names the first failed rule.
func ValidateRecommendation(r Recommendation, potML int) error {
	switch r.Confidence {
	case ConfidenceLow, ConfidenceMedium, ConfidenceHigh:
	default:
		return fmt.Errorf("confidence %q must be low, medium or high", r.Confidence)
	}
	if !r.NeedsWater && r.AmountML != 0 {
		return fmt.Errorf("amount_ml must be 0 when needs_water is false, got %d", r.AmountML)
	}
	if r.AmountML < 0 || r.AmountML > potML/2 {
		return fmt.Errorf("amount_ml %d outside 0..%d", r.AmountML, potML/2)
	}
	if n := len(r.Reason); n < 1 || n > maxReasonLen {
		return fmt.Errorf("reason length %d outside 1..%d", n, maxReasonLen)
	}
	return nil
}

// Disagree reports whether two recommendations differ materially: the needs_water
// verdict differs, or both water and one amount is more than twice the other.
func Disagree(a, b Recommendation) bool {
	if a.NeedsWater != b.NeedsWater {
		return true
	}
	if !a.NeedsWater {
		return false
	}
	lo, hi := min(a.AmountML, b.AmountML), max(a.AmountML, b.AmountML)
	return hi > 2*lo
}
