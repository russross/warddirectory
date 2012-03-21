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
	Name        string
	Label       string
	Filename    string
	CapHeight   int
	Glyphs      map[string]*GlyphMetrics
	Lookup      map[rune]string
	Extras      map[rune]string
	FirstChar   rune
	LastChar    rune
	Flags       int
	BBoxLeft    int
	BBoxBottom  int
	BBoxRight   int
	BBoxTop     int
	ItalicAngle int
	Ascent      int
	Descent     int
	StemV       int
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

var latin1mappings = map[rune]string{
	0x00A0: "nbspace",
	0x00A1: "exclamdown",
	0x00A2: "cent",
	0x00A3: "sterling",
	0x00A4: "currency",
	0x00A5: "yen",
	0x00A6: "brokenbar",
	0x00A7: "section",
	0x00A8: "dieresis",
	0x00A9: "copyright",
	0x00AA: "ordfeminine",
	0x00AB: "guillemotleft",
	0x00AC: "logicalnot",
	0x00AD: "softhyphen",
	0x00AE: "registered",
	0x00AF: "macron",
	0x00B0: "degree",
	0x00B1: "plusminus",
	0x00B2: "twosuperior",
	0x00B3: "threesuperior",
	0x00B4: "acute",
	0x00B5: "mu",
	0x00B6: "paragraph",
	0x00B7: "periodcentered",
	0x00B8: "cedilla",
	0x00B9: "onesuperior",
	0x00BA: "ordmasculine",
	0x00BB: "guillemotright",
	0x00BC: "onequarter",
	0x00BD: "onehalf",
	0x00BE: "threequarters",
	0x00BF: "questiondown",
	0x00C0: "Agrave",
	0x00C1: "Aacute",
	0x00C2: "Acircumflex",
	0x00C3: "Atilde",
	0x00C4: "Adieresis",
	0x00C5: "Aring",
	0x00C6: "AE",
	0x00C7: "Ccedilla",
	0x00C8: "Egrave",
	0x00C9: "Eacute",
	0x00CA: "Ecircumflex",
	0x00CB: "Edieresis",
	0x00CC: "Igrave",
	0x00CD: "Iacute",
	0x00CE: "Icircumflex",
	0x00CF: "Idieresis",
	0x00D0: "Eth",
	0x00D1: "Ntilde",
	0x00D2: "Ograve",
	0x00D3: "Oacute",
	0x00D4: "Ocircumflex",
	0x00D5: "Otilde",
	0x00D6: "Odieresis",
	0x00D7: "multiply",
	0x00D8: "Oslash",
	0x00D9: "Ugrave",
	0x00DA: "Uacute",
	0x00DB: "Ucircumflex",
	0x00DC: "Udieresis",
	0x00DD: "Yacute",
	0x00DE: "Thorn",
	0x00DF: "germandbls",
	0x00E0: "agrave",
	0x00E1: "aacute",
	0x00E2: "acircumflex",
	0x00E3: "atilde",
	0x00E4: "adieresis",
	0x00E5: "aring",
	0x00E6: "ae",
	0x00E7: "ccedilla",
	0x00E8: "egrave",
	0x00E9: "eacute",
	0x00EA: "ecircumflex",
	0x00EB: "edieresis",
	0x00EC: "igrave",
	0x00ED: "iacute",
	0x00EE: "icircumflex",
	0x00EF: "idieresis",
	0x00F0: "eth",
	0x00F1: "ntilde",
	0x00F2: "ograve",
	0x00F3: "oacute",
	0x00F4: "ocircumflex",
	0x00F5: "otilde",
	0x00F6: "odieresis",
	0x00F7: "divide",
	0x00F8: "oslash",
	0x00F9: "ugrave",
	0x00FA: "uacute",
	0x00FB: "ucircumflex",
	0x00FC: "udieresis",
	0x00FD: "yacute",
	0x00FE: "thorn",
	0x00FF: "ydieresis",
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
func ParseFontMetricsFile(file string, label string, stemv int) (font *FontMetrics, err error) {
	contents, err := ioutil.ReadFile(file)
	if err != nil {
		return
	}
	font = &FontMetrics{
		Glyphs: make(map[string]*GlyphMetrics),
		Lookup: make(map[rune]string),
		Label:  label,
		StemV:  stemv,
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
