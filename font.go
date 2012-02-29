package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"strings"
)

type GlyphMetrics struct {
	Code int
	Width int
	Name string
	BBoxLeft int
	BBoxBottom int
	BBoxRight int
	BBoxTop int
	Ligatures map[string]string
	Kerning map[string]int
}

type FontMetrics struct {
	Name string
	Glyphs map[string]*GlyphMetrics
	Lookup map[int]string
}

type Box struct {
	Font *FontMetrics
	Original string
	Width int
	Command string
}

func (font *FontMetrics) parseGlyph(in string) error {
	// sample: C 102 ; WX 333 ; N f ; B 20 0 383 683 ; L i fi ; L l fl ;
	glyph := &GlyphMetrics{ Ligatures: make(map[string]string), Kerning: make(map[string]int) }

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
			msg := fmt.Sprintf("MakeBox: Unknown character: [%c]", ch)
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
	for i, glyph := range glyphs {
		kern := 0
		if i+1 < len(glyphs) {
			kern = glyph.Kerning[glyphs[i+1].Name]
		}
		width += glyph.Width + kern

		switch {
		// simple glyphs
		case glyph.Code < 0x80 && kern != 0:
			pending += fmt.Sprintf("%c", glyph.Code)
			cmd += fmt.Sprintf("(%s)%d", pending, -kern)
			pending = ""
		case glyph.Code < 0x80:
			pending += fmt.Sprintf("%c", glyph.Code)

		// need to use a hex code for this glyph
		case pending != "" && kern != 0:
			cmd += fmt.Sprintf("(%s)<%02x>%d", pending, glyph.Code, -kern)
			pending = ""
		case pending != "":
			cmd += fmt.Sprintf("(%s)<%02x>", pending, glyph.Code)
			pending = ""
		case kern != 0:
			cmd += fmt.Sprintf("<%02x>%d", glyph.Code, -kern)
		default:
			cmd += fmt.Sprintf("<%02x>", glyph.Code)
		}
	}
	if pending != "" {
		cmd += fmt.Sprintf("(%s)", pending)
	}

	box = &Box{
		Font: font,
		Original: text,
		Width: width,
		Command: "[" + cmd + "] TJ",
	}
	return
}
