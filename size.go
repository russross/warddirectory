package main

import (
	"errors"
	"math"
)

const tolerance float64 = 0.01

var goldenratio float64 = (1.0 + math.Sqrt(5.0)) / 2.0

func (dir *Directory) findFontSize() (err error) {
	low, high := MinimumFontSize, MaximumFontSize
	finalrun := false
	success := false

	for {
		// get the next font size to try
		dir.FontSize = (high + low) / 2.0
		if finalrun {
			dir.FontSize = low
		}

		// the width of a single column
		// this is in font units, which are 1000ths of a point
		// adjusted for the font size
		firstlinewidth := dir.ColumnWidth * 1000.0 / dir.FontSize
		linewidth := firstlinewidth - goldenratio*1000.0
		spacesize := float64(dir.Roman.Glyphs["space"].Width)
		columnheight := dir.ColumnHeight * 1000.0 / dir.FontSize

		// do line breaking
		dir.Linebreaks = nil
		for _, entry := range dir.Entries {
			breaks := BreakParagraph(entry, firstlinewidth, linewidth, spacesize)
			dir.Linebreaks = append(dir.Linebreaks, breaks)
		}

		// do column breaking
		dir.Columnbreaks = BreakColumns(dir.Linebreaks, columnheight)

		if finalrun {
			break
		}

		// evaluate this break
		if len(dir.Columnbreaks) > dir.ColumnCount || len(dir.Columnbreaks) == 0 {
			high = dir.FontSize
		} else {
			low = dir.FontSize
			success = true
		}

		// are we finished?
		if high-low < tolerance {
			if low == dir.FontSize {
				break
			}

			// run it one more time to recompute the best one we found
			finalrun = true
		}
	}

	if !success {
		return errors.New("Unable to find a suitable font size")
	}
	return nil
}
