//
// Text processing
// Code to render strings using fonts,
// break entries into lines, break lists of entries into columns
// find the optimal font size, etc.
//

package main

import (
	"errors"
	"fmt"
	"math"
	"time"
)

// given a string, render it into PDF syntax using font metric data
// this also computes the width of the box in units equal to 1/1000th of a point
// if spacecompress != 1.0, space widths are adjusted by the given factor
func (font *FontMetrics) MakeBox(text string, spacecompress float64) (box *Box, err error) {
	// find the list of glyphs, merging ligatures when possible
	var glyphs []*GlyphMetrics
	for _, ch := range text {
		name, present := font.Lookup[int(ch)]
		if !present {
			msg := fmt.Sprintf("MakeBox: Unknown character: [%c] with code %d", ch, int(ch))
			return nil, errors.New(msg)
		}
		glyph := font.Glyphs[name]

		// see if this can be combined with the previous glyph
		count := len(glyphs)
		if count > 0 {
			if lig, present := glyphs[count-1].Ligatures[glyph.Name]; present {
				glyphs = glyphs[:count-1]
				glyph = font.Glyphs[lig]
			}
		}
		glyphs = append(glyphs, glyph)

		// record that this glyph has been used
		if glyph.Code > 0 {
			if font.FirstChar == 0 || glyph.Code < font.FirstChar {
				font.FirstChar = glyph.Code
			}
			if font.LastChar == 0 || glyph.Code > font.LastChar {
				font.LastChar = glyph.Code
			}
		}
	}

	// now compute the total width, including kerning
	var width float64
	cmd := ""
	pending := ""
	simple := true
	for i, glyph := range glyphs {
		var kern float64
		if i+1 < len(glyphs) {
			kern = float64(glyph.Kerning[glyphs[i+1].Name])
		}

		// do we need to "kern" this space to squish it?
		if spacecompress != 1.0 && glyph.Code == ' ' {
			kern -= float64(glyph.Width) - float64(glyph.Width)*spacecompress
		}
		width += float64(glyph.Width) + kern

		switch {
		// simple glyphs
		case glyph.Code < 0x80:
			switch glyph.Code {
			case '(':
				pending += "\\("
			case ')':
				pending += "\\)"
			case '\\':
				pending += "\\\\"
			default:
				pending += fmt.Sprintf("%c", glyph.Code)
			}
			if kern != 0 {
				if float64(int(kern)) == kern {
					cmd += fmt.Sprintf("(%s)%d", pending, -int(kern))
				} else {
					cmd += fmt.Sprintf("(%s)%.3f", pending, -kern)
				}
				pending = ""
				simple = false
			}

		// need to use a hex code for this glyph
		case pending != "" && kern != 0:
			cmd += fmt.Sprintf("(%s)<%02x>%d", pending, glyph.Code, -kern)
			pending = ""
			simple = false
		case pending != "":
			cmd += fmt.Sprintf("(%s)<%02x>", pending, glyph.Code)
			pending = ""
			simple = false
		case kern != 0:
			cmd += fmt.Sprintf("<%02x>%d", glyph.Code, -kern)
			simple = false
		default:
			cmd += fmt.Sprintf("<%02x>", glyph.Code)
			simple = false
		}
	}
	if pending != "" {
		cmd += fmt.Sprintf("(%s)", pending)
	}
	if simple {
		cmd = cmd + " Tj"
	} else {
		cmd = "[" + cmd + "] TJ"
	}

	box = &Box{
		Font:     font,
		Original: text,
		Width:    width,
		Command:  cmd,
	}
	return
}

// same as MakeBox, but panic in the event of an error
// useful in later passes, when all the text has already passed through MakeBox once
func (font *FontMetrics) MustMakeBox(text string, spacecompress float64) *Box {
	box, err := font.MakeBox(text, spacecompress)
	if err != nil {
		panic(err)
	}
	return box
}

type Breakable interface {
	Len() int
	Cost(a, b int, first, last bool) float64
}

type breakpoint struct {
	cost     float64 // best total cost of breaking this chunk
	nextline int     // start of next line
}

func Break(sequence Breakable) (startofeachchunk []int) {
	// the matrix of costs:
	//   matrix[from][to] = cost of breaking slice[from:to+1]
	dim := sequence.Len()
	backing := make([]breakpoint, dim*dim)
	matrix := make([][]breakpoint, dim)
	for i := 0; i < dim; i++ {
		matrix[i] = backing[i*dim : (i+1)*dim]
	}

	for from := dim - 1; from >= 0; from-- {
		for to := dim - 1; to >= from; to-- {
			// best = min(cost(from, i) + cost(i+1, to))
			matrix[from][to] = breakpoint{math.Inf(1), -1}
			for i := from; i <= to; i++ {
				cost := sequence.Cost(from, i+1, from == 0, i+1 == dim)
				if i+1 <= to {
					cost += matrix[i+1][to].cost
				}
				if cost < matrix[from][to].cost {
					matrix[from][to] = breakpoint{cost, i + 1}
				}
			}
		}
	}

	if math.IsInf(matrix[0][dim-1].cost, 1) || dim == 0 {
		return nil
	}

	startofeachchunk = nil
	for nextline := 0; nextline < dim; nextline = matrix[nextline][dim-1].nextline {
		startofeachchunk = append(startofeachchunk, nextline)
	}
	return
}

type BoxSlice struct {
	Boxes     []*Box
	Directory *Directory
}

func (elt *BoxSlice) Len() int {
	return len(elt.Boxes)
}

func (elt *BoxSlice) Cost(a, b int, first, last bool) float64 {
	words := elt.Boxes[a:b]

	// no space after the end of this sequence of words?
	if words[len(words)-1].JoinNext {
		return math.Inf(1)
	}

	// see if the line fits
	var spaces float64
	var cost float64
	for i, box := range words {
		cost += float64(box.Width)
		if !box.JoinNext && i+1 < len(words) {
			spaces += 1.0
		}
	}
	spacesize := float64(elt.Directory.Roman.Glyphs["space"].Width)
	maxwidth := cost + spaces*spacesize
	minwidth := cost + spaces*spacesize*elt.Directory.MinimumSpaceMultiplier

	// if we prefer not to break here, then the penalty is the
	// same as a completely blank line
	width := elt.Directory.ColumnWidth * 1000.0 / elt.Directory.FontSize
	if !first {
		width -= elt.Directory.FirstLineDedentMultiplier * 1000.0
	}
	penalty := width / spacesize
	penalty = penalty * penalty * float64(words[len(words)-1].Penalty)

	switch {
	// too long
	case minwidth > width:
		return math.Inf(1)

	// easy fit
	case maxwidth <= width:
		excess := (width - maxwidth) / spacesize

		// no penalty for trailing spaces on the last line
		if last {
			//excess = 0.0
		}

		return excess*excess + penalty

	// squished fit
	default:
		squish := (maxwidth - width) / spacesize
		return squish*squish*squish + penalty
	}
	panic("Can't get here")
}

type EntrySlice struct {
	Entries   [][]int
	Directory *Directory
}

func (elt *EntrySlice) Len() int {
	return len(elt.Entries)
}

func (elt *EntrySlice) Cost(a, b int, first, last bool) float64 {
	entries := elt.Entries[a:b]
	columnheight := elt.Directory.ColumnHeight * 1000.0 / elt.Directory.FontSize

	// count up the number of lines
	count := 0
	for _, entry := range entries {
		count += len(entry)
	}

	// units are normalized to one line per 1000 columnheight units
	squeeze := ((columnheight / 1000.0) - 1.0) / (float64(count-1) * elt.Directory.LeadingMultiplier)

	// forbid squeezing too much
	if squeeze < elt.Directory.MinimumLineHeightMultiplier {
		return math.Inf(1)
	}

	extralines := ((columnheight / 1000.0) - 1.0) -
		(float64(count-1) * elt.Directory.LeadingMultiplier)

	// squishing is worse than stretching
	if extralines < 0 {
		return -extralines * extralines * extralines
	}

	// how many lines worth?
	return extralines * extralines
}

func (dir *Directory) SplitIntoLines() {
	firstlinewidth := dir.ColumnWidth * 1000.0 / dir.FontSize
	linewidth := firstlinewidth - dir.FirstLineDedentMultiplier*1000.0
	for i, entry := range dir.Entries {
		var newentry [][]*Box
		breaks := dir.Linebreaks[i]
		for j, start := range breaks {
			var line []*Box
			if j+1 < len(breaks) {
				line = entry[start:breaks[j+1]]
			} else {
				line = entry[start:]
			}

			width := firstlinewidth
			if j > 0 {
				width = linewidth
			}

			line = dir.SimplifyLine(line, width)
			newentry = append(newentry, line)
		}

		dir.Lines = append(dir.Lines, newentry)
	}
}

// insert explicit spaces into a line
func (dir *Directory) SimplifyLine(boxes []*Box, linewidth float64) (simple []*Box) {
	// count up the spaces and the total line width
	var width, spaces float64
	for i, box := range boxes {
		width += float64(box.Width)
		if !box.JoinNext && i+1 < len(boxes) {
			spaces += 1
		}
	}
	spacefactor := float64(1.0)
	spacesize := float64(dir.Roman.Glyphs["space"].Width)
	maxwidth := width + spaces*spacesize

	if maxwidth > linewidth {
		// how much do we need to squeeze each space?
		extra := maxwidth - linewidth
		spacefactor = (spacesize - extra/spaces) / spacesize
	}

	for i := 0; i < len(boxes); i++ {
		box := boxes[i]

		// last box of the line?
		if i+1 == len(boxes) {
			simple = append(simple, box)
			break
		}

		next := boxes[i+1]

		switch {
		// nothing to be done
		case box.JoinNext && box.Font != next.Font:
			simple = append(simple, box)

		// simple merger
		case box.JoinNext:
			join := next.JoinNext
			boxes[i+1] = box.Font.MustMakeBox(box.Original+next.Original, spacefactor)
			boxes[i+1].JoinNext = join

		// same font with a space between
		case box.Font == next.Font:
			join := next.JoinNext
			boxes[i+1] = box.Font.MustMakeBox(box.Original+" "+next.Original, spacefactor)
			boxes[i+1].JoinNext = join

		// roman followed by anything
		case box.Font == dir.Roman:
			join := box.JoinNext
			box = box.Font.MustMakeBox(box.Original+" ", spacefactor)
			box.JoinNext = join
			simple = append(simple, box)

		// anything followed by roman
		case next.Font == dir.Roman:
			join := next.JoinNext
			boxes[i+1] = next.Font.MustMakeBox(" "+next.Original, spacefactor)
			boxes[i+1].JoinNext = join
			simple = append(simple, box)

		// bold followed by anything
		case box.Font == dir.Bold:
			join := box.JoinNext
			box = box.Font.MustMakeBox(box.Original+" ", spacefactor)
			box.JoinNext = join
			simple = append(simple, box)

		// anything followed by bold
		case next.Font == dir.Bold:
			join := next.JoinNext
			boxes[i+1] = next.Font.MustMakeBox(" "+next.Original, spacefactor)
			boxes[i+1].JoinNext = join
			simple = append(simple, box)

		default:
			panic("Can't get here")
		}
	}

	return
}

func (dir *Directory) RenderColumns() {
	// split the list of entries into columns
	for i, start := range dir.Columnbreaks {
		var column [][][]*Box
		if i+1 < len(dir.Columnbreaks) {
			column = dir.Lines[start:dir.Columnbreaks[i+1]]
		} else {
			column = dir.Lines[start:]
		}

		text := dir.RenderColumn(column, i%dir.ColumnsPerPage)
		dir.Columns = append(dir.Columns, text)
	}
}

func (dir *Directory) RenderColumn(entries [][][]*Box, number int) string {
	// find the top left corner
	x := dir.LeftMargin + (dir.ColumnWidth+dir.ColumnSep)*float64(number)
	y := dir.BottomMargin + dir.ColumnHeight - dir.FontSize

	// what is the starting position for an indented line?
	xi := x + dir.FontSize*dir.FirstLineDedentMultiplier

	// how many lines are there?
	count := 0
	for _, entry := range entries {
		count += len(entry)
	}

	// how tall must each line be to exactly fill the column?
	// strip off the top line, divide the remaining space evenly
	dy := (dir.ColumnHeight - dir.FontSize) / float64(count-1)

	// now walk through the entries and build each one
	rendered := "BT\n"
	for _, entry := range entries {
		elt := ""
		for i, line := range entry {
			if i == 0 {
				elt += fmt.Sprintf("1 0 0 1 %.3f %.3f Tm\n", x, y)
			} else {
				elt += fmt.Sprintf("1 0 0 1 %.3f %.3f Tm\n", xi, y)
			}
			y -= dy

			// render each box with its font
			for j, box := range line {
				if j > 0 {
					elt += " "
				}
				elt += fmt.Sprintf("/%s %.3f Tf ", box.Font.Label, dir.FontSize)
				elt += box.Command
			}

			elt += "\n"
		}

		rendered += elt
	}
	rendered += "ET\n"

	return rendered
}

func (dir *Directory) RenderHeader() (err error) {
	var title, date, useonly *Box
	if title, err = dir.Bold.MakeBox(dir.Title, 1.0); err != nil {
		return
	}
	if date, err = dir.Roman.MakeBox(time.Now().Format(dir.DateFormat), 1.0); err != nil {
		return
	}
	if useonly, err = dir.Roman.MakeBox(dir.Disclaimer, 1.0); err != nil {
		return
	}

	// figure out where the hrule goes
	length := dir.PageWidth - dir.RightMargin - dir.LeftMargin
	hrule := dir.PageHeight - dir.TopMargin
	y := hrule + dir.FontSize*(1.0-float64(dir.Roman.CapHeight)/1000.0)

	text := "0 g 0 G\n"
	text += "BT\n"

	// place the date
	text += fmt.Sprintf("1 0 0 1 %.3f %.3f Tm\n", dir.LeftMargin, y)
	text += fmt.Sprintf("/%s %.3f Tf %s\n", dir.Roman.Label, dir.FontSize, date.Command)

	// place the title
	tfontsize := dir.FontSize * dir.TitleFontMultiplier
	text += fmt.Sprintf("1 0 0 1 %.3f %.3f Tm\n",
		(dir.PageWidth-title.Width/1000.0*tfontsize)/2.0, y)
	text += fmt.Sprintf("/%s %.3f Tf %s\n", dir.Bold.Label, tfontsize, title.Command)

	// place the church-use-only text
	text += fmt.Sprintf("1 0 0 1 %.3f %.3f Tm\n",
		dir.PageWidth-dir.RightMargin-useonly.Width/1000.0*dir.FontSize, y)
	text += fmt.Sprintf("/%s %.3f Tf %s\n", dir.Roman.Label, dir.FontSize, useonly.Command)

	text += "ET\n"

	// place the hrule
	text += "q\n"
	text += fmt.Sprintf("1 0 0 1 %.3f %.3f cm\n", dir.LeftMargin, hrule)
	text += fmt.Sprintf("[]0 d 0 J 0.5 w 0 0 m %.3f 0 l s\n", length)
	text += "Q\n0 g 0 G\n"

	dir.Header = text

	return
}

func (dir *Directory) FindFontSize() (err error) {
	low, high := dir.MinimumFontSize, dir.MaximumFontSize
	finalrun := false
	success := false

	for {
		// get the next font size to try
		dir.FontSize = (high + low) / 2.0
		if finalrun {
			dir.FontSize = low
		}

		// do line breaking
		dir.Linebreaks = nil
		breaklines := &BoxSlice{
			Directory: dir,
		}
		for _, entry := range dir.Entries {
			breaklines.Boxes = entry
			breaks := Break(breaklines)
			dir.Linebreaks = append(dir.Linebreaks, breaks)
		}

		// do column breaking
		breakentries := &EntrySlice{
			Entries:   dir.Linebreaks,
			Directory: dir,
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
		if high-low < dir.FontSizePrecision {
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
