package main

import (
	"errors"
)

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
		linewidth := firstlinewidth - GoldenRatio*1000.0
		spacesize := float64(dir.Roman.Glyphs["space"].Width)
		columnheight := dir.ColumnHeight * 1000.0 / dir.FontSize

		// do line breaking
		dir.Linebreaks = nil
		breaklines := &BoxSlice{
			FirstLineWidth: firstlinewidth,
			LineWidth:      linewidth,
			SpaceSize:      spacesize,
		}
		for _, entry := range dir.Entries {
			breaklines.Boxes = entry
			breaks := Break(breaklines)
			dir.Linebreaks = append(dir.Linebreaks, breaks)
		}

		// do column breaking
		breakentries := &EntrySlice{
			Entries:      dir.Linebreaks,
			ColumnHeight: columnheight,
		}
		dir.Columnbreaks = Break(breakentries)

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
		if high-low < FontSizeTolerance {
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
