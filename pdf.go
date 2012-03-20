package main

import (
	"bytes"
	"compress/zlib"
	"encoding/json"
	"fmt"
)

const obj_info = `<<
  /Title (%s Directory)
  /Author (%s)
  /CreationDate (%s)
  /ModDate (%s)
  /Producer ()
  /Creator ()
  /Subject ()
  /Keywords ()
>>
`

const obj_catalog = `<<
  /Type /Catalog
  /Pages %s
>>
`

const obj_pages = `<<
  /Type /Pages
  /Kids [
    %s
    %s
  ]
  /Count 2
>>
`
const obj_page = `<<
  /Type /Page
  /MediaBox [0 0 %.3f %.3f]
  /Rotate 0
  /Parent %s
  /Resources <<
    /ProcSet [/PDF /ImageB /Text]
    /Font %s
  >>
  /Contents %s
>>
`

const obj_fontresource = `<<
  /FR %s
  /FB %s
  /FT %s
>>
`

const obj_font_builtin = `<<
  /Type /Font
  /Subtype /Type1
  /BaseFont %s
>>
`

const obj_font = `<<
  /Type /Font
  /Subtype /Type1
  /BaseFont %s
  /FirstChar %d
  /LastChar %d
  /Widths %s
  /FontDescriptor %s
>>
`

const obj_font_descriptor = `<<
  /Type /FontDescriptor
  /FontName %s
  /Flags %d
  /FontBBox [%d %d %d %d]
  /ItalicAngle %d
  /Ascent %d
  /Descent %d
  /CapHeight %d
  /StemV %d
>>
`

const obj_font_descriptor_embedded = `<<
  /Type /FontDescriptor
  /FontName %s
  /Flags %d
  /FontBBox [%d %d %d %d]
  /ItalicAngle %d
  /Ascent %d
  /Descent %d
  /CapHeight %d
  /StemV %d
  /FontFile %s
>>
`

const obj_page_stream = `<<
  /Length %d%s
>>
`

const obj_font_stream = `<<
  /Length1 %d
  /Length2 %d
  /Length3 %d
  /Length %%d%%s
>>
`

const obj_trailer = `trailer
<<
  /Size %d
  /Info %s
  /Root %s
>>
startxref
%d
%%%%EOF
`

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

type Document struct {
	out     *bytes.Buffer
	Xref    []int
	Trailer int
}

func NewDocument() *Document {
	elt := &Document{out: new(bytes.Buffer)}
	fmt.Fprint(elt.out, "%PDF-1.4\n%«»\n")
	return elt
}

func (elt *Document) AddObject(object string) (ref string) {
	offset := len(elt.out.Bytes())
	ref = elt.ForwardRef(0)
	elt.Xref = append(elt.Xref, offset)
	fmt.Fprintf(elt.out, "%d 0 obj\n%s", len(elt.Xref), object)
	fmt.Fprint(elt.out, "endobj\n")
	return
}

func (elt *Document) AddStream(object string, stream []byte) (ref string) {
	flate := ""
	if CompressStreams {
		flate = "\n  /Filter /FlateDecode"
		var compressed bytes.Buffer
		var writer *zlib.Writer
		var err error
		if writer, err = zlib.NewWriterLevel(&compressed, zlib.BestCompression); err != nil {
			panic(fmt.Sprint("Setting up zlib compressor: ", err))
		}
		if _, err = writer.Write(stream); err != nil {
			panic(fmt.Sprint("Writing to zlib compressor: ", err))
		}
		if err = writer.Close(); err != nil {
			panic(fmt.Sprint("Closing zlib compressor: ", err))
		}
		stream = compressed.Bytes()
	}

	offset := len(elt.out.Bytes())
	ref = elt.ForwardRef(0)
	elt.Xref = append(elt.Xref, offset)
	fmt.Fprintf(elt.out, "%d 0 obj\n%s", len(elt.Xref), fmt.Sprintf(object, len(stream), flate))
	fmt.Fprint(elt.out, "stream\n")
	elt.out.Write(stream)
	fmt.Fprint(elt.out, "endstream\n")
	fmt.Fprint(elt.out, "endobj\n")
	return
}

func (elt *Document) ForwardRef(inc int) (ref string) {
	return fmt.Sprintf("%d 0 R", len(elt.Xref)+1+inc)
}

func (elt *Document) WriteTrailer(info, catalog string) {
	startxref := len(elt.out.Bytes())
	fmt.Fprintf(elt.out, "xref\n0 %d\n0000000000 65535 f \n", len(elt.Xref)+1)
	for _, offset := range elt.Xref {
		fmt.Fprintf(elt.out, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(elt.out, obj_trailer, len(elt.Xref)+1, info, catalog, startxref)
}

func (elt *Document) Dump() {
	fmt.Print(elt.out.String())
}
