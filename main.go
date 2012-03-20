//
// Main driver
//

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"
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
	Disclaimer         = "For Church Use Only"
	CompressStreams    = true
)

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

	// fonts
	Roman      *FontMetrics `json:"-"`
	Bold       *FontMetrics `json:"-"`
	Typewriter *FontMetrics `json:"-"`

	// computed values
	ColumnWidth  float64 `json:"-"`
	ColumnHeight float64 `json:"-"`
	ColumnCount  int     `json:"-"`

	// processed values
	Families     []*Family  `json:"-"`
	Entries      [][]*Box   `json:"-"`
	Linebreaks   [][]int    `json:"-"`
	Columnbreaks []int      `json:"-"`
	Lines        [][][]*Box `json:"-"`
	FontSize     float64    `json:"-"`
	Columns      []string   `json:"-"`
	Header       string     `json:"-"`
}

func main() {
	// first load the fonts
	roman, err := parseFontMetricsFile(filepath.Join(fontPrefix, romanFont), "FR", romanStemV)
	if err != nil {
		log.Fatal("loading roman font: ", err)
	}
	bold, err := parseFontMetricsFile(filepath.Join(fontPrefix, boldFont), "FB", boldStemV)
	if err != nil {
		log.Fatal("loading bold font: ", err)
	}
	typewriter, err := parseFontMetricsFile(filepath.Join(fontPrefix, typewriterFont), "FT", typewriterStemV)
	if err != nil {
		log.Fatal("loading typewriter font: ", err)
	}
	typewriter.Filename = filepath.Join(fontPrefix, typewriterFontFile)

	// create directory object
	config, err := ioutil.ReadFile("config.json")
	if err != nil {
		log.Fatal("loading json file: ", err)
	}
	dir, err := NewDirectory(config, roman, bold, typewriter)
	if err != nil {
		log.Fatal("parsing json file: ", err)
	}

	// load and parse the families
	if err = dir.parseFamilies(os.Stdin); err != nil {
		log.Fatal("parsing families: ", err)
	}

	// format families
	if err = dir.formatFamilies(); err != nil {
		log.Fatal("formatting families: ", err)
	}

	// find the font size
	if err = dir.findFontSize(); err != nil {
		log.Fatal("finding font size: ", err)
	}

	// render the header
	if err = dir.renderHeader(); err != nil {
		log.Fatal("rendering header: ", err)
	}

	// render the family listings
	dir.splitIntoLines()
	dir.renderColumns()

	// generate the PDF file
	if err = dir.makePDF(); err != nil {
		log.Fatal("making the PDF: ", err)
	}
}

func NewDirectory(config []byte, roman, bold, typewriter *FontMetrics) (dir *Directory, err error) {

	dir = new(Directory)
	if err = json.Unmarshal(config, dir); err != nil {
		return
	}
	dir.Roman = roman
	dir.Bold = bold
	dir.Typewriter = typewriter

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

func (dir *Directory) makePDF() (err error) {
	// make the PDF file
	doc := NewDocument()

	timestamp := time.Now().Format("20060102150405-0700")
	timestamp = "D:" + timestamp[:17] + "'" + timestamp[17:19] + "'" + timestamp[19:]
	author := "Russ Ross"
	info := doc.AddObject(fmt.Sprintf(obj_info, dir.Title, author, timestamp, timestamp))
	pages := doc.ForwardRef(1)
	catalog := doc.AddObject(fmt.Sprintf(obj_catalog, pages))
	page1 := doc.ForwardRef(1)
	page2 := doc.ForwardRef(2)
	page1Contents := doc.ForwardRef(3)
	page2Contents := doc.ForwardRef(4)
	fontResource := doc.ForwardRef(5)
	doc.AddObject(fmt.Sprintf(obj_pages, page1, page2))
	doc.AddObject(fmt.Sprintf(obj_page, dir.PageWidth, dir.PageHeight, pages, fontResource, page1Contents))
	doc.AddObject(fmt.Sprintf(obj_page, dir.PageWidth, dir.PageHeight, pages, fontResource, page2Contents))

	// pages
	col := 0
	for page := 0; page < 2; page++ {
		text := dir.Header
		for i := 0; i < dir.ColumnsPerPage; i++ {
			text += dir.Columns[col]
			col++
		}
		doc.AddStream(obj_page_stream, []byte(text))
	}

	i := 1
	roman := doc.ForwardRef(i)
	i++
	var romanwidths, romandescriptor, romanembedded string
	if dir.Roman.Filename != "" {
		romanwidths = doc.ForwardRef(i)
		i++
		romandescriptor = doc.ForwardRef(i)
		i++
		romanembedded = doc.ForwardRef(i)
		i++
	}

	bold := doc.ForwardRef(i)
	i++
	var boldwidths, bolddescriptor, boldembedded string
	if dir.Bold.Filename != "" {
		boldwidths = doc.ForwardRef(i)
		i++
		bolddescriptor = doc.ForwardRef(i)
		i++
		boldembedded = doc.ForwardRef(i)
		i++
	}

	typewriter := doc.ForwardRef(i)
	i++
	var typewriterwidths, typewriterdescriptor, typewriterembedded string
	if dir.Typewriter.Filename != "" {
		typewriterwidths = doc.ForwardRef(i)
		i++
		typewriterdescriptor = doc.ForwardRef(i)
		i++
		typewriterembedded = doc.ForwardRef(i)
		i++
	}

	doc.AddObject(fmt.Sprintf(obj_fontresource, roman, bold, typewriter))

	// roman font
	doc.AddObject(makeFont(dir.Roman, romanwidths, romandescriptor))
	if dir.Roman.Filename != "" {
		doc.AddObject(makeWidths(dir.Roman))
		doc.AddObject(makeFontDescriptor(dir.Roman, romanembedded))
		var font []byte
		if font, err = ioutil.ReadFile(dir.Roman.Filename); err != nil {
			return
		}
		doc.AddStream(fmt.Sprintf(obj_font_stream, len(font), 0, 0), font)
	}

	// bold font
	doc.AddObject(makeFont(dir.Bold, boldwidths, bolddescriptor))
	if dir.Bold.Filename != "" {
		doc.AddObject(makeWidths(dir.Bold))
		doc.AddObject(makeFontDescriptor(dir.Bold, boldembedded))
		var font []byte
		if font, err = ioutil.ReadFile(dir.Bold.Filename); err != nil {
			return
		}
		doc.AddStream(fmt.Sprintf(obj_font_stream, len(font), 0, 0), font)
	}

	// typewriter font
	doc.AddObject(makeFont(dir.Typewriter, typewriterwidths, typewriterdescriptor))
	if dir.Typewriter.Filename != "" {
		doc.AddObject(makeWidths(dir.Typewriter))
		doc.AddObject(makeFontDescriptor(dir.Typewriter, typewriterembedded))
		var font []byte
		if font, err = ioutil.ReadFile(dir.Typewriter.Filename); err != nil {
			return
		}
		doc.AddStream(fmt.Sprintf(obj_font_stream, len(font), 0, 0), font)
	}

	doc.WriteTrailer(info, catalog)
	doc.Dump()
	return nil
}

func makeFont(font *FontMetrics, widths, descriptor string) string {
	if font.Filename == "" {
		return fmt.Sprintf(obj_font_builtin, "/"+font.Name)
	}
	return fmt.Sprintf(obj_font,
		"/"+font.Name,
		font.FirstChar,
		font.LastChar,
		widths,
		descriptor)
}

func makeWidths(font *FontMetrics) string {
	widths := "["
	for n := font.FirstChar; n <= font.LastChar; n++ {
		if n%16 == 0 {
			widths += "\n  "
		} else {
			widths += " "
		}
		if glyph, present := font.Lookup[n]; present {
			widths += fmt.Sprintf("%d", font.Glyphs[glyph].Width)
		} else {
			widths += "0"
		}
	}
	widths += "\n]\n"
	return widths
}

func makeFontDescriptor(font *FontMetrics, embedded string) string {
	if embedded == "" {
		// built in font
		return fmt.Sprintf(obj_font_descriptor,
			"/"+font.Name,
			font.Flags,
			font.BBoxLeft,
			font.BBoxBottom,
			font.BBoxRight,
			font.BBoxTop,
			font.ItalicAngle,
			font.Ascent,
			font.Descent,
			font.CapHeight,
			font.StemV)
	}
	return fmt.Sprintf(obj_font_descriptor_embedded,
		"/"+font.Name,
		font.Flags,
		font.BBoxLeft,
		font.BBoxBottom,
		font.BBoxRight,
		font.BBoxTop,
		font.ItalicAngle,
		font.Ascent,
		font.Descent,
		font.CapHeight,
		font.StemV,
		embedded)
}
