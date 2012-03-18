package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"strings"
	"time"
)

// The minimum allowed space size as a fraction of the normal size
const MinSpaceSize = .85
const MinLineHeight = .95
const Leading = 1.2

// How much worse is it to squish spaces than to pad the line with spaces?
const SquishedPenalty = 5.0

type GlyphMetrics struct {
	Code       int
	Width      int
	Name       string
	BBoxLeft   int
	BBoxBottom int
	BBoxRight  int
	BBoxTop    int
	Ligatures  map[string]string
	Kerning    map[string]int
}

type FontMetrics struct {
	Name      string
	Label     string
	CapHeight int
	Glyphs    map[string]*GlyphMetrics
	Lookup    map[int]string
}

type Box struct {
	Font     *FontMetrics
	Original string
	Width    float64
	Command  string
	JoinNext bool
	Penalty  int
}

func (font *FontMetrics) parseGlyph(in string) error {
	// sample: C 102 ; WX 333 ; N f ; B 20 0 383 683 ; L i fi ; L l fl ;
	glyph := &GlyphMetrics{Ligatures: make(map[string]string), Kerning: make(map[string]int)}

	for _, elt := range strings.Split(in, ";") {
		elt = strings.TrimSpace(elt)
		var a, b, c, d int
		var u, v string
		if n, err := fmt.Sscanf(elt, "C %d", &a); n == 1 && err == nil {
			glyph.Code = a
		} else if n, err := fmt.Sscanf(elt, "CH %x", &a); n == 1 && err == nil {
			glyph.Code = a
		} else if n, err := fmt.Sscanf(elt, "WX %d", &a); n == 1 && err == nil {
			glyph.Width = a
		} else if n, err := fmt.Sscanf(elt, "N %s", &u); n == 1 && err == nil {
			glyph.Name = u
		} else if n, err := fmt.Sscanf(elt, "B %d %d %d %d", &a, &b, &c, &d); n == 4 && err == nil {
			glyph.BBoxLeft, glyph.BBoxBottom, glyph.BBoxRight, glyph.BBoxTop = a, b, c, d
		} else if n, err := fmt.Sscanf(elt, "L %s %s", &u, &v); n == 2 && err == nil {
			glyph.Ligatures[u] = v
		} else if elt == "" {
		} else {
			return errors.New("Unknown glyph metric field: [" + elt + "] from [" + in + "]")
		}
	}

	if glyph.Name == "" {
		return errors.New("No glyph name found in metric line: [" + in + "]")
	}

	font.Glyphs[glyph.Name] = glyph
	if glyph.Code >= 0 {
		font.Lookup[glyph.Code] = glyph.Name
	}

	return nil
}

func (font *FontMetrics) parseKerning(in string) error {
	// sample: KPX f i -20
	var a int
	var u, v string
	if n, err := fmt.Sscanf(in, "KPX %s %s %d", &u, &v, &a); n == 3 && err == nil {
		glyph, present := font.Glyphs[u]
		if !present {
			return errors.New("Kerning found for unknown glyph: [" + in + "]")
		}
		glyph.Kerning[v] = a
	} else {
		return errors.New("Unknown kerning line: [" + in + "]")
	}

	return nil
}

func parseFontMetricsFile(file string) (font *FontMetrics, err error) {
	contents, err := ioutil.ReadFile(file)
	if err != nil {
		return
	}
	font = &FontMetrics{
		Glyphs: make(map[string]*GlyphMetrics),
		Lookup: make(map[int]string),
	}
	lines := strings.Split(string(contents), "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		count := 0
		n := 0
		s := ""
		if n, err = fmt.Sscanf(line, "CapHeight %d", &count); n == 1 && err == nil {
			font.CapHeight = count
		} else if n, err = fmt.Sscanf(line, "StartCharMetrics %d", &count); n == 1 && err == nil {
			i += 1
			for j := 0; j < count && i < len(lines); j, i = j+1, i+1 {
				line := strings.TrimSpace(lines[i])
				if err = font.parseGlyph(line); err != nil {
					return
				}
			}
		} else if n, err = fmt.Sscanf(line, "StartKernPairs %d", &count); n == 1 && err == nil {
			i += 1
			for j := 0; j < count && i < len(lines); i++ {
				line := strings.TrimSpace(lines[i])
				if line == "" {
					continue
				}
				if err = font.parseKerning(line); err != nil {
					return
				}
				j++
			}
		} else if n, err = fmt.Sscanf(line, "FontName %s", &s); n == 1 && err == nil {
			font.Name = s
		}
		err = nil
	}

	return
}

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

type breakpoint struct {
	cost     float64 // best total cost of breaking this chunk
	nextline int     // start of next line
}

func BreakParagraph(words []*Box, firstlinewidth, linewidth, spacesize float64) (startofeachline []int) {
	// the matrix of costs:
	//   matrix[from][to] = cost of breaking words[from:to+1]
	dim := len(words)
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
				width := linewidth
				if from == 0 {
					width = firstlinewidth
				}
				cost := LineCost(width, spacesize, words[from:i+1], i+1 == dim)
				if i+1 <= to {
					cost += matrix[i+1][to].cost
				}
				if cost < matrix[from][to].cost {
					matrix[from][to] = breakpoint{cost, i + 1}
				}
			}
		}
	}

	if math.IsInf(matrix[0][dim-1].cost, 1) || len(words) == 0 {
		return nil
	}
	//	for _, row := range matrix {
	//		for _, col := range row {
	//			fmt.Printf("%8.1f %3d ", col.cost, col.nextline)
	//		}
	//		fmt.Println()
	//	}

	startofeachline = nil
	for nextline := 0; nextline < dim; nextline = matrix[nextline][dim-1].nextline {
		startofeachline = append(startofeachline, nextline)
	}
	return
}

func LineCost(width, spacesize float64, words []*Box, lastline bool) (cost float64) {
	// no space after the end of this sequence of words?
	if words[len(words)-1].JoinNext {
		return math.Inf(1)
	}

	// see if the line fits
	var spaces float64
	for i, box := range words {
		cost += float64(box.Width)
		if !box.JoinNext && i+1 < len(words) {
			spaces += 1.0
		}
	}
	maxwidth := cost + spaces*spacesize
	minwidth := cost + spaces*spacesize*MinSpaceSize

	// if we prefer not to break here, then the penalty is the
	// same as a completely blank line
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
		if lastline {
			excess = 0.0
		}

		return excess*excess + penalty

	// squished fit
	default:
		squish := (maxwidth - width) / spacesize
		return squish*squish*SquishedPenalty + penalty
	}
	panic("Can't get here")
}

func BreakColumns(entries [][]int, columnheight float64) (startofeachcolumn []int) {
	// the matrix of costs:
	//   matrix[from][to] = cost of breaking entries[from:to+1]
	dim := len(entries)
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
				cost := ColumnCost(columnheight, entries[from:i+1])
				if i+1 <= to {
					cost += matrix[i+1][to].cost
				}
				if cost < matrix[from][to].cost {
					matrix[from][to] = breakpoint{cost, i + 1}
				}
			}
		}
	}

	if math.IsInf(matrix[0][dim-1].cost, 1) || len(entries) == 0 {
		return nil
	}

	startofeachcolumn = nil
	for nextline := 0; nextline < dim; nextline = matrix[nextline][dim-1].nextline {
		startofeachcolumn = append(startofeachcolumn, nextline)
	}
	return
}

func ColumnCost(columnheight float64, entries [][]int) float64 {
	// count up the number of lines
	count := 0
	for _, entry := range entries {
		count += len(entry)
	}

	// units are normalized to one line per 1000 columnheight units
	squeeze := ((columnheight / 1000.0) - 1.0) / (float64(count-1) * Leading)

	// forbid squeezing too much
	if squeeze < MinLineHeight {
		return math.Inf(1)
	}

	extralines := columnheight/1000.0 - float64(count)

	// how many lines worth?
	return extralines * extralines
}

func (dir *Directory) splitIntoLines() (err error) {
	firstlinewidth := dir.ColumnWidth * 1000.0 / dir.FontSize
	linewidth := firstlinewidth - goldenratio*1000.0
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

			if line, err = dir.simplifyLine(line, width); err != nil {
				return
			}

			newentry = append(newentry, line)
		}

		dir.Lines = append(dir.Lines, newentry)
	}

	return
}

// insert explicit spaces into a line
func (dir *Directory) simplifyLine(boxes []*Box, linewidth float64) (simple []*Box, err error) {
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
			if boxes[i+1], err = box.Font.MakeBox(box.Original+next.Original, spacefactor); err != nil {
				return
			}
			boxes[i+1].JoinNext = join

		// same font with a space between
		case box.Font == next.Font:
			join := next.JoinNext
			if boxes[i+1], err = box.Font.MakeBox(box.Original+" "+next.Original, spacefactor); err != nil {
				return
			}
			boxes[i+1].JoinNext = join

		// roman followed by anything
		case box.Font == dir.Roman:
			join := box.JoinNext
			if box, err = box.Font.MakeBox(box.Original+" ", spacefactor); err != nil {
				return
			}
			box.JoinNext = join
			simple = append(simple, box)

		// anything followed by roman
		case next.Font == dir.Roman:
			join := next.JoinNext
			if boxes[i+1], err = next.Font.MakeBox(" "+next.Original, spacefactor); err != nil {
				return
			}
			boxes[i+1].JoinNext = join
			simple = append(simple, box)

		// bold followed by anything
		case box.Font == dir.Bold:
			join := box.JoinNext
			if box, err = box.Font.MakeBox(box.Original+" ", spacefactor); err != nil {
				return
			}
			box.JoinNext = join
			simple = append(simple, box)

		// anything followed by bold
		case next.Font == dir.Bold:
			join := next.JoinNext
			if boxes[i+1], err = next.Font.MakeBox(" "+next.Original, spacefactor); err != nil {
				return
			}
			boxes[i+1].JoinNext = join
			simple = append(simple, box)

		default:
			panic("Can't get here")
		}
	}

	return
}

func (dir *Directory) renderColumns() (err error) {
	// split the list of entries into columns
	for i, start := range dir.Columnbreaks {
		var column [][][]*Box
		if i+1 < len(dir.Columnbreaks) {
			column = dir.Lines[start:dir.Columnbreaks[i+1]]
		} else {
			column = dir.Lines[start:]
		}

		var text string
		if text, err = dir.renderColumn(column, i%dir.ColumnsPerPage); err != nil {
			return
		}

		dir.Columns = append(dir.Columns, text)
	}

	return
}

func (dir *Directory) renderColumn(entries [][][]*Box, number int) (rendered string, err error) {
	// find the top left corner
	x := dir.LeftMargin + (dir.ColumnWidth+dir.ColumnSep)*float64(number)
	y := dir.BottomMargin + dir.ColumnHeight - dir.FontSize

	// what is the starting position for an indented line?
	xi := x + dir.FontSize*goldenratio

	// how many lines are there?
	count := 0
	for _, entry := range entries {
		count += len(entry)
	}

	// how tall must each line be to exactly fill the column?
	// strip off the top line, divide the remaining space evenly
	dy := -(dir.ColumnHeight - dir.FontSize) / float64(count-1)

	// now walk through the entries and build each one
	rendered = "BT\n"
	for _, entry := range entries {
		elt := ""
		for i, line := range entry {
			if i == 0 {
				elt += fmt.Sprintf("1 0 0 1 %.3f %.3f Tm\n", x, y)
			} else {
				elt += fmt.Sprintf("1 0 0 1 %.3f %.3f Tm\n", xi, y)
			}
			y += dy

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

	return
}

func (dir *Directory) renderHeader() (err error) {
	var title, date, useonly *Box
	if title, err = dir.Bold.MakeBox(dir.Title, 1.0); err != nil {
		return
	}
	if date, err = dir.Roman.MakeBox(time.Now().Format("January 2, 2006"), 1.0); err != nil {
		return
	}
	if useonly, err = dir.Roman.MakeBox(ForChurchUseOnly, 1.0); err != nil {
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
	tfontsize := dir.FontSize * TitleFontMultiplier
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
