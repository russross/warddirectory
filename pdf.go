package main

import (
	"bytes"
	"fmt"
	"io"
	"time"
)

type Value interface {
	Render(out io.Writer, indent string)
}

type Object map[string]Value

func (elt Object) Render(out io.Writer, indent string) {
	fmt.Fprintln(out, "<<")
	for key, val := range elt {
		fmt.Fprintln(out, "/%s ", key)
		val.Render(out, indent + "  ")
	}
	fmt.Fprintln(out, "%s>>\n", indent)
}

type Str string

func (elt *Str) Render(out io.Writer, indent string) {
	fmt.Fprintf(out, "(%s)\n", elt)
}

type Ref struct {
	Count int
	Actual Value
}

func (elt *Ref) Render(out io.Writer, indent string) {
	fmt.Fprintf(out, "%d 0 R\n", elt.Count)
}

const obj_info = `<<
  /Title (%s)
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
  /Encoding <<
	/Type /Encoding
	/Differences [
		128 /fi
	]
  >>
>>
`

const obj_page_stream = `<<
  /Length %d
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

type Offset uint

type Document struct {
	out *bytes.Buffer
	Xref []Offset
	Trailer Offset
}

func New() *Document {
	elt := &Document{ out: new(bytes.Buffer) }
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
	offset := Offset(len(elt.out.Bytes()))
	ref = elt.ForwardRef(0)
	elt.Xref = append(elt.Xref, offset)
	fmt.Fprintf(elt.out, "%d 0 obj\n%s", len(elt.Xref), fmt.Sprintf(object, len(stream)))
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

func main() {
	doc := New()

	timestamp := time.Now().Format("20060102150405-0700")
	timestamp = "D:" + timestamp[:17] + "'" + timestamp[17:19] + "'" + timestamp[19:]
	title := "Diamond Valley Second Ward"
	author := "Russ Ross"
	info := doc.AddObject(fmt.Sprintf(obj_info, title, author, timestamp, timestamp))
	pages := doc.ForwardRef(1)
	catalog := doc.AddObject(fmt.Sprintf(obj_catalog, pages))
	page1 := doc.ForwardRef(1)
	page2 := doc.ForwardRef(2)
	fontResource := doc.ForwardRef(3)
	page1Contents := doc.ForwardRef(7)
	page2Contents := doc.ForwardRef(8)
	doc.AddObject(fmt.Sprintf(obj_pages, page1, page2))
	doc.AddObject(fmt.Sprintf(obj_page, pages, fontResource, page1Contents))
	doc.AddObject(fmt.Sprintf(obj_page, pages, fontResource, page2Contents))
	roman := doc.ForwardRef(1)
	bold := doc.ForwardRef(2)
	typewriter := doc.ForwardRef(3)
	doc.AddObject(fmt.Sprintf(obj_fontresource, roman, bold, typewriter))
	doc.AddObject(fmt.Sprintf(obj_font, "/Times-Roman"))
	doc.AddObject(fmt.Sprintf(obj_font, "/Times-Bold"))
	doc.AddObject(fmt.Sprintf(obj_font, "/Courier"))
	doc.AddStream(obj_page_stream, []byte(makepage1()))
	doc.WriteTrailer(info, catalog)
	//doc.Dump()
	font, err := parseFontMetricsFile("/usr/share/texmf-texlive/fonts/afm/adobe/times/ptmr8a.afm")
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("Font: %s\n", font.Name)
//	for key, val := range font.Glyphs {
//		fmt.Printf("%s: %v\n", key, val)
//	}
	box, err := font.MakeBox("Yes, find me a sandwich. ")
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("%#v\n", box)
}

func makepage1() string {
	return `1 0 0 1 36 727 Tm
/FB 20 Tf
(Ross) Tj
/FR 20 Tf
( Russ \(773-5952, ) Tj
/FT 20 Tf
(russ@russross.com) Tj
/FR 20 Tf
(\),) Tj
1 0 0 1 52 707 Tm
[(Nanc)15(y \(773-5953, )] TJ
/FT 20 Tf
(nancy@nancyross.com) Tj
/FR 20 Tf
(\), Rosie, Alex,) Tj
1 0 0 1 52 687 Tm
(1414 Agate Ct) Tj
1 0 0 1 52 667 Tm
[(Y)100(es, )<ae>(nd me a sandwich.)] TJ
1 0 0 1 251.08 667 Tm
([End])Tj
`
}
