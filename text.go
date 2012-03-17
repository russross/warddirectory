package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"strings"
)

// The minimum allowed space size as a fraction of the normal size
const MinSpaceSize = .75
const MinLineHeight = .95

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
	Name   string
	Glyphs map[string]*GlyphMetrics
	Lookup map[int]string
}

type Box struct {
	Font     *FontMetrics
	Original string
	Width    int
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
		if n, err = fmt.Sscanf(line, "StartCharMetrics %d", &count); n == 1 && err == nil {
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

func (font *FontMetrics) MakeBox(text string) (box *Box, err error) {
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
	width := 0
	cmd := ""
	pending := ""
	simple := true
	for i, glyph := range glyphs {
		kern := 0
		if i+1 < len(glyphs) {
			kern = glyph.Kerning[glyphs[i+1].Name]
		}
		width += glyph.Width + kern

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
				cmd += fmt.Sprintf("(%s)%d", pending, -kern)
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
		return squish*SquishedPenalty + penalty
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
	//	for _, row := range matrix {
	//		for _, col := range row {
	//			fmt.Printf("%8.1f %3d ", col.cost, col.nextline)
	//		}
	//		fmt.Println()
	//	}

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
	squeeze := (columnheight / 1000.0) / float64(count)

	// forbid squeezing too much
	if squeeze < MinLineHeight {
		return math.Inf(1)
	}

	extralines := columnheight/1000.0 - float64(count)

	// squeezing is worse than stretching
	if extralines < 0 {
		return extralines * extralines * SquishedPenalty
	}

	// how many lines worth?
	return extralines * extralines
}
