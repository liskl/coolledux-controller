package models

// DrawItem represents a single pixel with its color and position in a
// drawing sequence. Color is stored as RGB888 (0xRRGGBB).
type DrawItem struct {
	Color         uint32 `json:"color"`
	SequenceIndex int    `json:"sequence_index"`
}
