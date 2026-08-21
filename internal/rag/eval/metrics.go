package eval

import "github.com/google/uuid"

// PrecisionAtK = |relevant ∩ retrieved[:k]| / k. k <= 0 yields 0.
func PrecisionAtK(relevant, retrieved []uuid.UUID, k int) float64 {
	if k <= 0 {
		return 0
	}
	relSet := toSet(relevant)
	hits := 0
	for i, id := range retrieved {
		if i >= k {
			break
		}
		if relSet[id] {
			hits++
		}
	}
	return float64(hits) / float64(k)
}

// RecallAtK = |relevant ∩ retrieved[:k]| / |relevant|, or 0 when relevant is empty.
func RecallAtK(relevant, retrieved []uuid.UUID, k int) float64 {
	if len(relevant) == 0 {
		return 0
	}
	relSet := toSet(relevant)
	hits := 0
	for i, id := range retrieved {
		if i >= k {
			break
		}
		if relSet[id] {
			hits++
		}
	}
	return float64(hits) / float64(len(relevant))
}

// MRR = 1/rank of the first relevant retrieved item (1-based), or 0 if none.
func MRR(relevant, retrieved []uuid.UUID) float64 {
	relSet := toSet(relevant)
	for i, id := range retrieved {
		if relSet[id] {
			return 1.0 / float64(i+1)
		}
	}
	return 0
}

func toSet(ids []uuid.UUID) map[uuid.UUID]bool {
	m := make(map[uuid.UUID]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m
}
