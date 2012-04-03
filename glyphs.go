package main

import (
	"fmt"
	"strconv"
	"strings"
)

var windows1252mappings map[rune]string

func parseGlyphList(known map[string]bool, universal map[string]bool, file string) (mapping map[rune]string, err error) {
	mapping = make(map[rune]string)

	// process one line at a time
	for _, line := range strings.Split(file, "\n") {
		// skip comment/blank lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, ";")
		if len(fields) != 2 {
			return nil, fmt.Errorf("Wrong number of fields in glyph list line: [%s]", line)
		}
		name := strings.TrimSpace(fields[0])
		if !known[name] {
			continue
		}
		for _, code := range strings.Fields(fields[1]) {
			var n uint64
			if n, err = strconv.ParseUint(code, 16, 16); err != nil {
				return nil, fmt.Errorf("Invalid code in glyph list line: [%s]: %v", line, err)
			}
			r := rune(n)

			if _, present := mapping[r]; present {
				// prefer a glyph that is know by all fonts
				// if both are know by all, then it doesn't matter
				// if neither is know by all, we pick one arbitrarily
				if universal[name] {
					mapping[r] = name
				}
			} else {
				mapping[r] = name
			}
		}
	}

	return
}

func GlyphMapping(lst map[string]*FontMetrics, glyphlist string) (mapping map[rune]string, err error) {
	union := make(map[string]bool)
	intersection := make(map[string]bool)
	for name, _ := range lst["times-roman"].Glyphs {
		intersection[name] = true
	}

	for _, font := range lst {
		// get the union of all the glyph names we know about
		for name, _ := range font.Glyphs {
			union[name] = true
		}

		// get the intersection, too
		for name, _ := range intersection {
			if _, present := font.Glyphs[name]; !present {
				delete(intersection, name)
			}
		}
	}

	// now use those to inform the unicode -> glyph mapping
	if mapping, err = parseGlyphList(union, intersection, glyphlist); err != nil {
		return
	}

	// map in Windows1252 code points just in case
	windows1252 := map[rune]rune{
		0x80: 0x20ac,
		0x82: 0x201a,
		0x83: 0x0192,
		0x84: 0x201e,
		0x85: 0x2026,
		0x86: 0x2020,
		0x87: 0x2021,
		0x88: 0x02c6,
		0x89: 0x2030,
		0x8a: 0x0160,
		0x8b: 0x2039,
		0x8c: 0x0152,
		0x8e: 0x017d,
		0x91: 0x2018,
		0x92: 0x2019,
		0x93: 0x201c,
		0x94: 0x201d,
		0x95: 0x2022,
		0x96: 0x2013,
		0x97: 0x2014,
		0x98: 0x02dc,
		0x99: 0x2122,
		0x9a: 0x0161,
		0x9b: 0x203a,
		0x9c: 0x0153,
		0x9e: 0x017e,
		0x9f: 0x0178,
	}

	for windows, unicode := range windows1252 {
		if name, present := mapping[unicode]; present {
			mapping[windows] = name
		}
	}

	return
}
