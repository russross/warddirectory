//
// Font metric files
// Code to parse and represent font metrics
//

package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"strings"
)

// metrics for a single glyph from a font
type GlyphMetrics struct {
	Code       rune
	Width      int
	Name       string
	BBoxLeft   int
	BBoxBottom int
	BBoxRight  int
	BBoxTop    int
	Ligatures  map[string]string
	Kerning    map[string]int
}

// metrics for an entire font
type FontMetrics struct {
	Name           string
	Label          string
	File           []byte
	CompressedFile []byte
	CapHeight      int
	Glyphs         map[string]*GlyphMetrics
	Lookup         map[rune]string
	FirstChar      rune
	LastChar       rune
	Flags          int
	BBoxLeft       int
	BBoxBottom     int
	BBoxRight      int
	BBoxTop        int
	ItalicAngle    int
	Ascent         int
	Descent        int
	StemV          int
}

// a single chunk of text made up of glyphs
type Box struct {
	Font     *FontMetrics
	Original string
	Width    float64
	Command  string
	JoinNext bool
	Penalty  int
}

// parse a single glyph metric line from a .afm file
func (font *FontMetrics) ParseGlyph(in string) error {
	// sample: C 102 ; WX 333 ; N f ; B 20 0 383 683 ; L i fi ; L l fl ;
	glyph := &GlyphMetrics{Ligatures: make(map[string]string), Kerning: make(map[string]int)}

	for _, elt := range strings.Split(in, ";") {
		elt = strings.TrimSpace(elt)
		var a, b, c, d int
		var r rune
		var u, v string
		if n, err := fmt.Sscanf(elt, "C %d", &r); n == 1 && err == nil {
			glyph.Code = r
		} else if n, err := fmt.Sscanf(elt, "CH %x", &r); n == 1 && err == nil {
			glyph.Code = r
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

	if _, present := font.Glyphs[glyph.Name]; present {
		panic("Duplicate glyph found while parsing font metrics file")
	}
	font.Glyphs[glyph.Name] = glyph
	if glyph.Code >= 0 {
		font.Lookup[glyph.Code] = glyph.Name
	}

	return nil
}

// parse a single glyph kerning line from a .afm file
func (font *FontMetrics) ParseKerning(in string) error {
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

// parse and entire .afm file
func ParseFontMetricsFile(file string, label string) (font *FontMetrics, err error) {
	contents, err := ioutil.ReadFile(file)
	if err != nil {
		return
	}
	font = &FontMetrics{
		Glyphs: make(map[string]*GlyphMetrics),
		Lookup: make(map[rune]string),
		Label:  label,
		Flags:  1<<1 | 1<<5,
	}
	lines := strings.Split(string(contents), "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		a, b, c, d, count := 0, 0, 0, 0, 0
		n := 0
		s := ""
		if n, err = fmt.Sscanf(line, "CapHeight %d", &a); n == 1 && err == nil {
			font.CapHeight = a
		} else if n, err = fmt.Sscanf(line, "FontBBox %d %d %d %d", &a, &b, &c, &d); n == 4 && err == nil {
			font.BBoxLeft = a
			font.BBoxBottom = b
			font.BBoxRight = c
			font.BBoxTop = d
		} else if n, err = fmt.Sscanf(line, "ItalicAngle %d", &a); n == 1 && err == nil {
			font.ItalicAngle = a
		} else if n, err = fmt.Sscanf(line, "Ascender %d", &a); n == 1 && err == nil {
			font.Ascent = a
		} else if n, err = fmt.Sscanf(line, "Descender %d", &a); n == 1 && err == nil {
			font.Descent = a
		} else if n, err = fmt.Sscanf(line, "StdVW %d", &a); n == 1 && err == nil {
			font.StemV = a
		} else if n, err = fmt.Sscanf(line, "IsFixedPitch %s", &s); n == 1 && err == nil {
			if s == "true" {
				font.Flags |= 1
			}
		} else if n, err = fmt.Sscanf(line, "StartCharMetrics %d", &count); n == 1 && err == nil {
			i += 1
			for j := 0; j < count && i < len(lines); j, i = j+1, i+1 {
				line := strings.TrimSpace(lines[i])
				if err = font.ParseGlyph(line); err != nil {
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
				if err = font.ParseKerning(line); err != nil {
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

func (font *FontMetrics) Copy() *FontMetrics {
	elt := new(FontMetrics)
	*elt = *font
	elt.Lookup = make(map[rune]string)
	for _, glyph := range elt.Glyphs {
		if glyph.Code > 0 {
			elt.Lookup[glyph.Code] = glyph.Name
		}
	}
	elt.FirstChar = 0
	elt.LastChar = 0
	return elt
}
