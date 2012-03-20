//
// Main driver
//

package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
)

const (
	outputfilename     = "directory.pdf"
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

	Regexp *regexp.Regexp `json:"-"`
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

	PhoneRegexps   []*RegularExpression
	AddressRegexps []*RegularExpression

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
	// where is the input coming from?
	in := os.Stdin
	if len(os.Args) == 2 {
		var err error
		in, err = os.Open(os.Args[1])
		if err != nil {
			log.Fatal("opening ["+os.Args[1]+"]: ", err)
		}
	}

	// first load the fonts
	roman, err := ParseFontMetricsFile(filepath.Join(fontPrefix, romanFont), "FR", romanStemV)
	if err != nil {
		log.Fatal("loading roman font: ", err)
	}
	bold, err := ParseFontMetricsFile(filepath.Join(fontPrefix, boldFont), "FB", boldStemV)
	if err != nil {
		log.Fatal("loading bold font: ", err)
	}
	typewriter, err := ParseFontMetricsFile(filepath.Join(fontPrefix, typewriterFont), "FT", typewriterStemV)
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
	if err = dir.CompileRegexps(); err != nil {
		log.Fatal("bad regular expression in config file: ", err)
	}

	// load and parse the families
	if err = dir.ParseFamilies(in); err != nil {
		log.Fatal("parsing families: ", err)
	}
	if len(os.Args) == 2 {
		in.Close()
	}

	// format families
	if err = dir.FormatFamilies(); err != nil {
		log.Fatal("formatting families: ", err)
	}

	// find the font size
	if err = dir.FindFontSize(); err != nil {
		log.Fatal("finding font size: ", err)
	}

	// render the header
	if err = dir.RenderHeader(); err != nil {
		log.Fatal("rendering header: ", err)
	}

	// render the family listings
	dir.SplitIntoLines()
	dir.RenderColumns()

	// generate the PDF file
	if err = dir.MakePDF(); err != nil {
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
