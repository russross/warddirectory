//
// Main directory data structure
//

package main

import (
	"regexp"
	"strings"
	"time"
)

const (
	CompressStreams            = true
	FallbackGlyph              = "question"
	FallbackTypewriter         = "courier"
	FontSizePrecision          = 0.01
	StartingFontSize   float64 = 10.0
	MinimumFontSize    float64 = 1.0
	MaximumFontSize    float64 = 100.0
	Subject                    = "LDS Ward Directory"
	Creator                    = "https://lds.org/directory/"
	Producer                   = "http://russross.github.com/warddirectory/"
)

type RegularExpression struct {
	Expression  string
	Replacement string

	Regexp *regexp.Regexp `json:"-" schema:"-"`
}

type Directory struct {
	// configured values
	Title                       string
	DateFormat                  string
	Disclaimer                  string
	TitleFontSize               float64
	HeaderFontSize              float64
	FooterLeft                  string
	FooterCenter                string
	FooterRight                 string
	FooterFontSize              float64
	PageWidth                   float64
	PageHeight                  float64
	TopMargin                   float64
	BottomMargin                float64
	LeftMargin                  float64
	RightMargin                 float64
	Pages                       int
	ColumnsPerPage              int
	ColumnSep                   float64
	EmailFont                   string
	LeadingMultiplier           float64
	MinimumSpaceMultiplier      float64
	MinimumLineHeightMultiplier float64
	FirstLineDedentMultiplier   float64
	FullFamily                  bool
	FamilyPhone                 bool
	FamilyEmail                 bool
	FamilyAddress               bool
	PersonalPhones              bool
	PersonalEmails              bool
	UseAmpersand                bool

	PhoneRegexps   []*RegularExpression
	AddressRegexps []*RegularExpression
	NameRegexps    []*RegularExpression

	// fonts
	Roman      *FontMetrics `json:"-" schema:"-"`
	Bold       *FontMetrics `json:"-" schema:"-"`
	Typewriter *FontMetrics `json:"-" schema:"-"`

	// computed values
	ColumnWidth  float64 `json:"-" schema:"-"`
	ColumnHeight float64 `json:"-" schema:"-"`
	ColumnCount  int     `json:"-" schema:"-"`

	// processed values
	Families     []*Family  `json:"-" schema:"-"`
	Entries      [][]*Box   `json:"-" schema:"-"`
	Linebreaks   [][]int    `json:"-" schema:"-"`
	Columnbreaks []int      `json:"-" schema:"-"`
	Lines        [][][]*Box `json:"-" schema:"-"`
	FontSize     float64    `json:"-" schema:"-"`
	Columns      []string   `json:"-" schema:"-"`
	Header       string     `json:"-" schema:"-"`
	Footer       string     `json:"-" schema:"-"`
	Author       string     `json:"-" schema:"-"`

	// part of the HTML form, we ignore it
	SubmitButton string `json:"-"`
}

func (dir *Directory) Copy() *Directory {
	elt := new(Directory)
	*elt = *dir

	// clone the regexps
	kinds := []*[]*RegularExpression{
		&dir.PhoneRegexps,
		&dir.AddressRegexps,
		&dir.NameRegexps,
	}
	for _, kind := range kinds {
		old := *kind
		*kind = nil
		for _, re := range old {
			re2 := new(RegularExpression)
			*re2 = *re
			re2.Regexp = nil
			*kind = append(*kind, re2)
		}
	}

	elt.Roman = dir.Roman.Copy()
	elt.Bold = dir.Bold.Copy()
	//elt.Typewriter = dir.Typewriter.Copy()

	// clear all the processed values
	elt.Families = nil
	elt.Entries = nil
	elt.Linebreaks = nil
	elt.Columnbreaks = nil
	elt.Lines = nil
	elt.FontSize = 0.0
	elt.Columns = nil
	elt.Header = ""
	elt.Footer = ""
	elt.Author = ""

	return elt
}

func (dir *Directory) ComputeImplicitFields() {
	dir.ColumnCount = dir.ColumnsPerPage * dir.Pages
	dir.ColumnWidth = dir.PageWidth
	dir.ColumnWidth -= dir.LeftMargin
	dir.ColumnWidth -= dir.RightMargin
	dir.ColumnWidth -= dir.ColumnSep * float64(dir.ColumnsPerPage-1)
	dir.ColumnWidth /= float64(dir.ColumnsPerPage)
	dir.ColumnHeight = dir.PageHeight
	dir.ColumnHeight -= dir.TopMargin
	dir.ColumnHeight -= dir.BottomMargin

	// remove empty regexps
	kinds := []*[]*RegularExpression{
		&dir.PhoneRegexps,
		&dir.AddressRegexps,
		&dir.NameRegexps,
	}
	for _, kind := range kinds {
		old := *kind
		*kind = nil
		for _, re := range old {
			if strings.TrimSpace(re.Expression) != "" {
				*kind = append(*kind, re)
			}
		}
	}
}

// build a complete PDF object for this directory
func (dir *Directory) MakePDF() (pdf []byte, err error) {
	// make the PDF file
	var doc Document

	// build the info section
	mst := time.FixedZone("MST", -7*3600)
	timestamp := time.Now().In(mst).Format("20060102150405-0700")
	timestamp = "D:" + timestamp[:17] + "'" + timestamp[17:19] + "'" + timestamp[19:]
	info := PDFMap{
		"Title":        PDFString(dir.Title + " Directory"),
		"Author":       PDFString(dir.Author),
		"Subject":      PDFString(Subject),
		"Creator":      PDFString(Creator),
		"Producer":     PDFString(Producer),
		"CreationDate": PDFString(timestamp),
		"ModDate":      PDFString(timestamp),
	}
	info_ref := doc.TopLevelObject(info)

	// build the root catalog
	catalog := PDFMap{
		"Type": PDFName("Catalog"),
	}
	catalog_ref := doc.TopLevelObject(catalog)

	// the list of fonts, shared by all pages
	fontResource := PDFMap{}
	fontResource_ref := doc.TopLevelObject(fontResource)

	// build the fonts
	// only add fonts that were actually used
	var roman_ref, bold_ref, typewriter_ref PDFRef
	if dir.Roman.LastChar > 0 {
		if roman_ref, err = doc.MakeFont(dir.Roman); err != nil {
			return
		}
		fontResource["FR"] = roman_ref
	}
	if dir.Bold.LastChar > 0 {
		if bold_ref, err = doc.MakeFont(dir.Bold); err != nil {
			return
		}
		fontResource["FB"] = bold_ref
	}
	if dir.Typewriter.LastChar > 0 {
		if typewriter_ref, err = doc.MakeFont(dir.Typewriter); err != nil {
			return
		}
		fontResource["FT"] = typewriter_ref
	}

	// build the list of pages
	kids := PDFSlice(nil)
	pages := PDFMap{
		"Type":  PDFName("Pages"),
		"Count": PDFNumber(dir.Pages),
	}
	pages_ref := doc.TopLevelObject(pages)
	catalog["Pages"] = pages_ref

	// build the actual page objects
	col := 0
	for i := 0; i < dir.Pages; i++ {
		// first get the contents of this page
		text := dir.Header
		for i := 0; i < dir.ColumnsPerPage; i++ {
			text += dir.Columns[col]
			col++
		}
		text += dir.Footer
		contents := &PDFStream{
			Map:  PDFMap{},
			Data: []byte(text),
		}
		contents_ref := doc.TopLevelObject(contents)

		page := PDFMap{
			"Type": PDFName("Page"),
			"MediaBox": PDFSlice{
				PDFNumber(0),
				PDFNumber(0),
				PDFNumber(dir.PageWidth),
				PDFNumber(dir.PageHeight),
			},
			"Rotate": PDFNumber(0),
			"Parent": pages_ref,
			"Resources": PDFMap{
				"ProcSet": PDFSlice{
					PDFName("PDF"),
					PDFName("ImageB"),
					PDFName("Text"),
				},
				"Font": fontResource_ref,
			},
			"Contents": contents_ref,
		}
		page_ref := doc.TopLevelObject(page)
		kids = append(kids, page_ref)
	}
	pages["Kids"] = kids

	return doc.Render(info_ref, catalog_ref)
}
