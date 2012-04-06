//
// Main directory data structure
//

package main

import (
	"github.com/russross/warddirectory/data"
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
	Producer                   = "http://ward-directory.appspot.com/"
)

type fontdata struct {
	Metrics  string
	FontFile string
	Label    string
	StemV    int
}

var FontSourceList = map[string]*fontdata{
	"times-roman": {data.Times_Roman_afm, "", "FR", -1},
	"times-bold":  {data.Times_Bold_afm, "", "FB", -1},
	"courier":     {data.Courier_afm, "", "FT", -1},
	"lmtt":        {data.Lmtt10_afm, data.Lmtt10_pfb, "FT", 69},
	"lmvtt":       {data.Lmvtt10_afm, data.Lmvtt10_pfb, "FT", 69},
}

type RegularExpression struct {
	Expression  string
	Replacement string

	Regexp *regexp.Regexp `json:"-" schema:"-" datastore:"-"`
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

	PhoneRegexps   []*RegularExpression `datastore:"-"`
	AddressRegexps []*RegularExpression `datastore:"-"`
	NameRegexps    []*RegularExpression `datastore:"-"`

	PhoneExpressions    []string `json:"-" schema:"-"`
	PhoneReplacements   []string `json:"-" schema:"-"`
	AddressExpressions  []string `json:"-" schema:"-"`
	AddressReplacements []string `json:"-" schema:"-"`
	NameExpressions     []string `json:"-" schema:"-"`
	NameReplacements    []string `json:"-" schema:"-"`

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
	Footer       string     `json:"-" schema:"-" datastore:"-"`
	Author       string     `json:"-" schema:"-" datastore:"-"`
}

func (dir *Directory) Copy() *Directory {
	elt := new(Directory)
	*elt = *dir

	// clone the regexps
	elt.ToDatastore()
	elt.FromDatastore()

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
	dir.NameRegexps = nil
	for i := 0; i < len(dir.NameExpressions) && i < len(dir.NameReplacements); i++ {
		re := &RegularExpression{
			Expression:  dir.NameExpressions[i],
			Replacement: dir.NameReplacements[i],
		}
		dir.NameRegexps = append(dir.NameRegexps, re)
	}
}

func (dir *Directory) ToDatastore() {
	// convert the regexps to datastore-friendly format
	dir.PhoneExpressions = nil
	dir.PhoneReplacements = nil
	for _, re := range dir.PhoneRegexps {
		if strings.TrimSpace(re.Expression) == "" {
			// skip empty regexps
			continue
		}
		dir.PhoneExpressions = append(dir.PhoneExpressions, re.Expression)
		dir.PhoneReplacements = append(dir.PhoneReplacements, re.Replacement)
	}
	dir.AddressExpressions = nil
	dir.AddressReplacements = nil
	for _, re := range dir.AddressRegexps {
		if strings.TrimSpace(re.Expression) == "" {
			// skip empty regexps
			continue
		}
		dir.AddressExpressions = append(dir.AddressExpressions, re.Expression)
		dir.AddressReplacements = append(dir.AddressReplacements, re.Replacement)
	}
	dir.NameExpressions = nil
	dir.NameReplacements = nil
	for _, re := range dir.NameRegexps {
		if strings.TrimSpace(re.Expression) == "" {
			// skip empty regexps
			continue
		}
		dir.NameExpressions = append(dir.NameExpressions, re.Expression)
		dir.NameReplacements = append(dir.NameReplacements, re.Replacement)
	}
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
