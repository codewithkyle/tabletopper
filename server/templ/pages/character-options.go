package pages

import "tabletopper/internal/dnd"

type Option = dnd.Option

var (
	alignmentOptions = dnd.Alignments()
	sizeOptions      = dnd.Sizes()
)
