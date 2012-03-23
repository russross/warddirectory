package main

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"strings"
	"time"
)

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

func (elt *Document) AddStream(object string, stream []byte, compressed []byte) (ref string) {
	flate := ""
	if CompressStreams {
		flate = "\n  /Filter /FlateDecode"
		if len(compressed) == 0 {
			var buf bytes.Buffer
			var writer *zlib.Writer
			var err error
			if writer, err = zlib.NewWriterLevel(&buf, zlib.BestCompression); err != nil {
				panic(fmt.Sprint("Setting up zlib compressor: ", err))
			}
			if _, err = writer.Write(stream); err != nil {
				panic(fmt.Sprint("Writing to zlib compressor: ", err))
			}
			if err = writer.Close(); err != nil {
				panic(fmt.Sprint("Closing zlib compressor: ", err))
			}
			stream = buf.Bytes()
		}
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

func (dir *Directory) MakePDF() (pdf []byte, err error) {
	// make the PDF file
	doc := NewDocument()

	mst := time.FixedZone("MST", -7*3600)
	timestamp := time.Now().In(mst).Format("20060102150405-0700")
	timestamp = "D:" + timestamp[:17] + "'" + timestamp[17:19] + "'" + timestamp[19:]
	author := "Russ Ross"
	info := doc.AddObject(fmt.Sprintf(obj_info, dir.Title, author, timestamp, timestamp))
	pages := doc.ForwardRef(1)
	catalog := doc.AddObject(fmt.Sprintf(obj_catalog, pages))
	fwd := 1
	var pagelst []string
	for i := 0; i < dir.Pages; i++ {
		pagelst = append(pagelst, doc.ForwardRef(fwd))
		fwd++
	}
	var contents []string
	for i := 0; i < dir.Pages; i++ {
		contents = append(contents, doc.ForwardRef(fwd))
		fwd++
	}
	fontResource := doc.ForwardRef(fwd)
	fwd++
	doc.AddObject(fmt.Sprintf(obj_pages, strings.Join(pagelst, "\n    "), dir.Pages))
	for i := 0; i < dir.Pages; i++ {
		doc.AddObject(fmt.Sprintf(obj_page, dir.PageWidth, dir.PageHeight, pages, fontResource, contents[i]))
	}

	// pages
	col := 0
	for page := 0; page < dir.Pages; page++ {
		text := dir.Header
		for i := 0; i < dir.ColumnsPerPage; i++ {
			text += dir.Columns[col]
			col++
		}
		doc.AddStream(obj_page_stream, []byte(text), nil)
	}

	i := 1
	roman := doc.ForwardRef(i)
	i++
	var romanwidths, romandescriptor, romanembedded string
	if len(dir.Roman.File) > 0 {
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
	if len(dir.Bold.File) > 0 {
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
	if len(dir.Typewriter.File) > 0 {
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
	if len(dir.Roman.File) > 0 {
		doc.AddObject(makeWidths(dir.Roman))
		doc.AddObject(makeFontDescriptor(dir.Roman, romanembedded))
		doc.AddStream(fmt.Sprintf(obj_font_stream, len(dir.Roman.File), 0, 0),
			dir.Roman.File, dir.Roman.CompressedFile)
	}

	// bold font
	doc.AddObject(makeFont(dir.Bold, boldwidths, bolddescriptor))
	if len(dir.Bold.File) > 0 {
		doc.AddObject(makeWidths(dir.Bold))
		doc.AddObject(makeFontDescriptor(dir.Bold, boldembedded))
		doc.AddStream(fmt.Sprintf(obj_font_stream, len(dir.Bold.File), 0, 0),
			dir.Bold.File, dir.Bold.CompressedFile)
	}

	// typewriter font
	doc.AddObject(makeFont(dir.Typewriter, typewriterwidths, typewriterdescriptor))
	if len(dir.Typewriter.File) > 0 {
		doc.AddObject(makeWidths(dir.Typewriter))
		doc.AddObject(makeFontDescriptor(dir.Typewriter, typewriterembedded))
		doc.AddStream(fmt.Sprintf(obj_font_stream, len(dir.Typewriter.File), 0, 0),
			dir.Typewriter.File, dir.Typewriter.CompressedFile)
	}

	doc.WriteTrailer(info, catalog)
	return doc.out.Bytes(), nil
}

func makeFont(font *FontMetrics, widths, descriptor string) string {
	if len(font.File) == 0 {
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
  ]
  /Count %d
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
