//
// Main directory data structure
//

package main

import (
	"encoding/json"
	"regexp"
)

const (
	fontPrefix         = "fonts"
	romanFont          = "ptmr8a.afm"
	romanStemV         = 85 // this is missing from the .afm file
	boldFont           = "ptmb8a.afm"
	boldStemV          = 140
	typewriterFont     = "cmtt10.afm"
	typewriterFontFile = "cmtt10.pfb"
	typewriterStemV    = 125
	CompressStreams    = true
)

type RegularExpression struct {
	Expression  string
	Replacement string

	Regexp *regexp.Regexp `json:"-" schema:"-" datastore:"-"`
}

type Directory struct {
	// configured values
	Title                       string
	Disclaimer                  string
	DateFormat                  string
	PageWidth                   float64
	PageHeight                  float64
	TopMargin                   float64
	BottomMargin                float64
	LeftMargin                  float64
	RightMargin                 float64
	ColumnsPerPage              int
	ColumnSep                   float64
	LeadingMultiplier           float64
	MinimumFontSize             float64
	MaximumFontSize             float64
	MinimumSpaceMultiplier      float64
	MinimumLineHeightMultiplier float64
	FontSizePrecision           float64
	TitleFontMultiplier         float64
	FirstLineDedentMultiplier   float64

	PhoneRegexps   []*RegularExpression `datastore:"-"`
	AddressRegexps []*RegularExpression `datastore:"-"`

	PhoneExpressions    []string `json:"-" schema:"-"`
	PhoneReplacements   []string `json:"-" schema:"-"`
	AddressExpressions  []string `json:"-" schema:"-"`
	AddressReplacements []string `json:"-" schema:"-"`

	// fonts
	Roman      *FontMetrics `json:"-" schema:"-" datastore:"-"`
	Bold       *FontMetrics `json:"-" schema:"-" datastore:"-"`
	Typewriter *FontMetrics `json:"-" schema:"-" datastore:"-"`

	// computed values
	ColumnWidth  float64 `json:"-" schema:"-" datastore:"-"`
	ColumnHeight float64 `json:"-" schema:"-" datastore:"-"`
	ColumnCount  int     `json:"-" schema:"-" datastore:"-"`

	// processed values
	Families     []*Family  `json:"-" schema:"-" datastore:"-"`
	Entries      [][]*Box   `json:"-" schema:"-" datastore:"-"`
	Linebreaks   [][]int    `json:"-" schema:"-" datastore:"-"`
	Columnbreaks []int      `json:"-" schema:"-" datastore:"-"`
	Lines        [][][]*Box `json:"-" schema:"-" datastore:"-"`
	FontSize     float64    `json:"-" schema:"-" datastore:"-"`
	Columns      []string   `json:"-" schema:"-" datastore:"-"`
	Header       string     `json:"-" schema:"-" datastore:"-"`
}

func (dir *Directory) FromDatastore() {
	// convert the regexps from datastore format to normal
	dir.PhoneRegexps = nil
	for i := 0; i < len(dir.PhoneExpressions) && i < len(dir.PhoneReplacements); i++ {
		re := &RegularExpression{
			Expression:  dir.PhoneExpressions[i],
			Replacement: dir.PhoneReplacements[i],
		}
		dir.PhoneRegexps = append(dir.PhoneRegexps, re)
	}
	dir.AddressRegexps = nil
	for i := 0; i < len(dir.AddressExpressions) && i < len(dir.AddressReplacements); i++ {
		re := &RegularExpression{
			Expression:  dir.AddressExpressions[i],
			Replacement: dir.AddressReplacements[i],
		}
		dir.AddressRegexps = append(dir.AddressRegexps, re)
	}
}

func (dir *Directory) ToDatastore() {
	// convert the regexps to datastore-friendly format
	dir.PhoneExpressions = nil
	dir.PhoneReplacements = nil
	for _, re := range dir.PhoneRegexps {
		dir.PhoneExpressions = append(dir.PhoneExpressions, re.Expression)
		dir.PhoneReplacements = append(dir.PhoneReplacements, re.Replacement)
	}
	dir.AddressExpressions = nil
	dir.AddressReplacements = nil
	for _, re := range dir.AddressRegexps {
		dir.AddressExpressions = append(dir.AddressExpressions, re.Expression)
		dir.AddressReplacements = append(dir.AddressReplacements, re.Replacement)
	}
}

func (dir *Directory) Prepare(roman, bold, typewriter *FontMetrics) {
	dir.Roman = roman.Copy()
	dir.Bold = bold.Copy()
	dir.Typewriter = typewriter.Copy()

	dir.ColumnCount = dir.ColumnsPerPage * 2
	dir.ColumnWidth = dir.PageWidth
	dir.ColumnWidth -= dir.LeftMargin
	dir.ColumnWidth -= dir.RightMargin
	dir.ColumnWidth -= dir.ColumnSep * float64(dir.ColumnsPerPage-1)
	dir.ColumnWidth /= float64(dir.ColumnsPerPage)
	dir.ColumnHeight = dir.PageHeight
	dir.ColumnHeight -= dir.TopMargin
	dir.ColumnHeight -= dir.BottomMargin
}

func NewDirectory(config []byte, roman, bold, typewriter *FontMetrics) (dir *Directory, err error) {
	dir = new(Directory)
	if err = json.Unmarshal(config, dir); err != nil {
		return
	}
	dir.Roman = roman.Copy()
	dir.Bold = bold.Copy()
	dir.Typewriter = typewriter.Copy()

	dir.ColumnCount = dir.ColumnsPerPage * 2
	dir.ColumnWidth = dir.PageWidth
	dir.ColumnWidth -= dir.LeftMargin
	dir.ColumnWidth -= dir.RightMargin
	dir.ColumnWidth -= dir.ColumnSep * float64(dir.ColumnsPerPage-1)
	dir.ColumnWidth /= float64(dir.ColumnsPerPage)
	dir.ColumnHeight = dir.PageHeight
	dir.ColumnHeight -= dir.TopMargin
	dir.ColumnHeight -= dir.BottomMargin

	return
}
