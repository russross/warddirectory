package main

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"math"
)

type Value interface {
	Render(out io.Writer, indent string)
}

type Object map[string]Value

func (elt Object) Render(out io.Writer, indent string) {
	fmt.Fprintln(out, "<<")
	for key, val := range elt {
		fmt.Fprintln(out, "/%s ", key)
		val.Render(out, indent+"  ")
	}
	fmt.Fprintln(out, "%s>>\n", indent)
}

type Str string

func (elt *Str) Render(out io.Writer, indent string) {
	fmt.Fprintf(out, "(%s)\n", elt)
}

type Ref struct {
	Count  int
	Actual Value
}

func (elt *Ref) Render(out io.Writer, indent string) {
	fmt.Fprintf(out, "%d 0 R\n", elt.Count)
}

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
  /MediaBox [0 0 612 792]
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

const obj_trailer = `trailer
<<
  /Size %d
  /Info %s
  /Root %s
>>
startxref
%d
%%EOF
`

const (
	inch            float64 = 72.0
	MinimumFontSize float64 = 4.0
	MaximumFontSize float64 = 18.0
)

var TitleFontMultiplier float64 = math.Sqrt(2.0)

type Directory struct {
	PageWidth, PageHeight       float64
	TopMargin, BottomMargin     float64
	LeftMargin, RightMargin     float64
	ColumnsPerPage, ColumnCount int
	ColumnSep                   float64
	ColumnWidth, ColumnHeight   float64

	Roman, Bold, Typewriter *FontMetrics

	Title        string
	Families     []*Family
	Entries      [][]*Box
	Linebreaks   [][]int
	Columnbreaks []int
	Lines        [][][]*Box
	FontSize     float64
	Columns      []string
	Header       string
}

func NewDirectory(title string, roman, bold, typewriter *FontMetrics) *Directory {
	elt := &Directory{
		PageWidth:      8.5 * inch,
		PageHeight:     11.0 * inch,
		TopMargin:      1.0 * inch, // note: header is in top margin
		BottomMargin:   .75 * inch,
		LeftMargin:     .5 * inch,
		RightMargin:    .5 * inch,
		ColumnsPerPage: 2,
		ColumnSep:      10.0,

		Roman:      roman,
		Bold:       bold,
		Typewriter: typewriter,

		Title: title,
	}
	elt.ColumnCount = elt.ColumnsPerPage * 2
	elt.ColumnWidth = elt.PageWidth
	elt.ColumnWidth -= elt.LeftMargin
	elt.ColumnWidth -= elt.RightMargin
	elt.ColumnWidth -= elt.ColumnSep * float64(elt.ColumnsPerPage-1)
	elt.ColumnWidth /= float64(elt.ColumnsPerPage)
	elt.ColumnHeight = elt.PageHeight
	elt.ColumnHeight -= elt.TopMargin
	elt.ColumnHeight -= elt.BottomMargin

	return elt
}

type Offset uint

type Document struct {
	out     *bytes.Buffer
	Xref    []Offset
	Trailer Offset
}

func NewDocument() *Document {
	elt := &Document{out: new(bytes.Buffer)}
	fmt.Fprint(elt.out, "%PDF-1.4\n%«»\n")
	return elt
}

func (elt *Document) AddObject(object string) (ref string) {
	offset := Offset(len(elt.out.Bytes()))
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
			panic(fmt.Sprint("Setting up zlip compressor: ", err))
		}
		if _, err = writer.Write(stream); err != nil {
			panic(fmt.Sprint("Writing to zlip compressor: ", err))
		}
		if err = writer.Close(); err != nil {
			panic(fmt.Sprint("Closing zlip compressor: ", err))
		}
		stream = compressed.Bytes()
	}

	offset := Offset(len(elt.out.Bytes()))
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
	startxref := Offset(len(elt.out.Bytes()))
	fmt.Fprintf(elt.out, "xref\n0 %d\n0000000000 65535 f \n", len(elt.Xref)+1)
	for _, offset := range elt.Xref {
		fmt.Fprintf(elt.out, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(elt.out, obj_trailer, len(elt.Xref)+1, info, catalog, startxref)
}

func (elt *Document) Dump() {
	fmt.Print(elt.out.String())
}
