package ui

import "math"

type scrollLayout struct {
	top         int
	height      int
	total       int
	offset      int
	thumbStart  int
	thumbHeight int
}

func newScrollLayout(top, desiredHeight, total, offset int) scrollLayout {
	height := clampViewportHeight(desiredHeight)
	offset = clampScrollOffset(offset, total, height)
	thumbStart, thumbHeight := scrollbarThumb(total, height, offset)
	return scrollLayout{
		top:         top,
		height:      height,
		total:       total,
		offset:      offset,
		thumbStart:  thumbStart,
		thumbHeight: thumbHeight,
	}
}

func clampViewportHeight(desired int) int {
	if desired < 5 {
		return 5
	}
	return desired
}

func clampScrollOffset(offset, total, viewportHeight int) int {
	maxOffset := total - viewportHeight
	if maxOffset < 0 {
		maxOffset = 0
	}
	if offset < 0 {
		return 0
	}
	if offset > maxOffset {
		return maxOffset
	}
	return offset
}

func scrollbarThumb(total, viewportHeight, offset int) (int, int) {
	if total <= viewportHeight || viewportHeight <= 0 {
		return 0, viewportHeight
	}

	thumbHeight := int(math.Round(float64(viewportHeight*viewportHeight) / float64(total)))
	if thumbHeight < 1 {
		thumbHeight = 1
	}
	if thumbHeight > viewportHeight {
		thumbHeight = viewportHeight
	}

	maxOffset := total - viewportHeight
	maxThumbStart := viewportHeight - thumbHeight
	if maxOffset <= 0 || maxThumbStart <= 0 {
		return 0, thumbHeight
	}

	ratio := float64(offset) / float64(maxOffset)
	thumbStart := int(math.Round(ratio * float64(maxThumbStart)))
	if thumbStart < 0 {
		thumbStart = 0
	}
	if thumbStart > maxThumbStart {
		thumbStart = maxThumbStart
	}

	return thumbStart, thumbHeight
}

func scrollOffsetFromThumb(total, viewportHeight, thumbStart int) int {
	if total <= viewportHeight || viewportHeight <= 0 {
		return 0
	}

	_, thumbHeight := scrollbarThumb(total, viewportHeight, 0)
	maxThumbStart := viewportHeight - thumbHeight
	if maxThumbStart <= 0 {
		return 0
	}

	if thumbStart < 0 {
		thumbStart = 0
	}
	if thumbStart > maxThumbStart {
		thumbStart = maxThumbStart
	}

	maxOffset := total - viewportHeight
	return int(math.Round(float64(thumbStart) / float64(maxThumbStart) * float64(maxOffset)))
}

func scrollbarHitMinX(width int) int {
	if width <= 1 {
		return 0
	}
	return width - 2
}
