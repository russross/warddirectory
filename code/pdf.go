package main

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"strings"
)

// any object that can be rendered in a PDF file
type PDFObject interface {
	Render(indent string, out io.Writer) (err error)
}

// strings like (this)
type PDFString string

func (s PDFString) Render(indent string, out io.Writer) (err error) {
	fmt.Fprintf(out, "(%s)", s)
	return
}

// strings like /this
type PDFName string

func (s PDFName) Render(indent string, out io.Writer) (err error) {
	fmt.Fprintf(out, "/%s", s)
	return
}

// indirect object references
type PDFRef string

func (s PDFRef) Render(indent string, out io.Writer) (err error) {
	fmt.Fprintf(out, "%s", s)
	return
}

// any number, float or integer
type PDFNumber float64

func (n PDFNumber) Render(indent string, out io.Writer) (err error) {
	if strings.Contains(fmt.Sprintf("%g", n), "e") {
		fmt.Fprintf(out, "%f", n)
	} else {
		fmt.Fprintf(out, "%g", n)
	}
	return
}

// a list of PDF objects [ like this ]
type PDFSlice []PDFObject

func (object PDFSlice) Render(indent string, out io.Writer) (err error) {
	fmt.Fprint(out, "[\n")
	for _, elt := range object {
		fmt.Fprintf(out, "%s  ", indent)
		if err = elt.Render(indent+"  ", out); err != nil {
			return err
		}
		fmt.Fprintln(out)
	}
	fmt.Fprintf(out, "%s]", indent)
	return
}

// a map of PDF objects << /Like: this >>
type PDFMap map[string]PDFObject

func (object PDFMap) Render(indent string, out io.Writer) (err error) {
	fmt.Fprint(out, "<<\n")
	for key, val := range object {
		fmt.Fprintf(out, "%s  /%s ", indent, key)
		if err = val.Render(indent+"  ", out); err != nil {
			return
		}
		fmt.Fprintln(out)
	}
	fmt.Fprintf(out, "%s>>", indent)
	return
}

// a PDF stream with optional compression
type PDFStream struct {
	Map        PDFMap
	Data       []byte
	Compressed []byte
}

func (stream *PDFStream) Render(indent string, out io.Writer) (err error) {
	if CompressStreams && len(stream.Compressed) == 0 {
		// compress the stream contents
		var buf bytes.Buffer
		var writer *zlib.Writer
		if writer, err = zlib.NewWriterLevel(&buf, zlib.BestCompression); err != nil {
			return
		}
		if _, err = writer.Write(stream.Data); err != nil {
			return
		}
		if err = writer.Close(); err != nil {
			return
		}
		stream.Compressed = buf.Bytes()
	}
	if CompressStreams {
		stream.Map["Filter"] = PDFName("FlateDecode")
		stream.Map["Length"] = PDFNumber(len(stream.Compressed))
	} else {
		stream.Map["Length"] = PDFNumber(len(stream.Data))
	}
	if err = stream.Map.Render(indent, out); err != nil {
		return
	}
	fmt.Fprint(out, "\nstream\n")
	if CompressStreams {
		if _, err = out.Write(stream.Compressed); err != nil {
			return
		}
	} else {
		if _, err = out.Write(stream.Data); err != nil {
			return
		}
	}
	fmt.Fprint(out, "endstream")
	return
}

// a slice of widths, rendered more compactly than a general slice
type PDFWidthSlice []int

func (slice PDFWidthSlice) Render(indent string, out io.Writer) (err error) {
	fmt.Fprint(out, "[")
	for n, width := range slice {
		if n%16 == 0 {
			fmt.Fprintf(out, "\n%s  ", indent)
		} else {
			fmt.Fprint(out, " ")
		}
		fmt.Fprintf(out, "%d", width)
	}
	fmt.Fprintf(out, "\n%s]", indent)
	return
}

// a top-level PDF document
type Document []PDFObject

func (doc *Document) TopLevelObject(elt PDFObject) (ref PDFRef) {
	*doc = append(*doc, elt)
	return PDFRef(fmt.Sprintf("%d 0 R", len(*doc)))
}

func (doc Document) Render(info, catalog PDFRef) (pdf []byte, err error) {
	var out bytes.Buffer
	var xref []int

	fmt.Fprint(&out, "%PDF-1.4\n%«»\n")
	for i, elt := range doc {
		xref = append(xref, out.Len())
		fmt.Fprintf(&out, "%d 0 obj\n", i+1)
		if err = elt.Render("", &out); err != nil {
			return
		}
		fmt.Fprint(&out, "\nendobj\n")
	}

	startxref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(xref)+1)
	for _, offset := range xref {
		fmt.Fprintf(&out, "%010d 00000 n \n", offset)
	}
	fmt.Fprint(&out, "trailer\n<<\n")
	fmt.Fprintf(&out, "  /Size %d\n", len(xref)+1)
	fmt.Fprintf(&out, "  /Info %s\n", string(info))
	fmt.Fprintf(&out, "  /Root %s\n", string(catalog))
	fmt.Fprintf(&out, ">>\nstartxref\n%d\n%%%%EOF\n", startxref)

	return out.Bytes(), nil
}

func (doc *Document) MakeFont(font *FontMetrics) (ref PDFRef, err error) {
	var fontobject PDFMap

	// built in font
	if len(font.File) == 0 {
		fontobject = PDFMap{
			"Type":     PDFName("Font"),
			"Subtype":  PDFName("Type1"),
			"BaseFont": PDFName(font.Name),
		}

	} else {
		// build the list of widths
		widths := PDFWidthSlice(nil)
		for n := font.FirstChar; n <= font.LastChar; n++ {
			if name, present := font.CodePointToName[n]; present {
				widths = append(widths, font.Glyphs[name].Width)
			} else {
				widths = append(widths, 0)
			}
		}

		// embed the font file
		file := &PDFStream{
			Map: PDFMap{
				"Length1": PDFNumber(len(font.File)),
				"Length2": PDFNumber(0),
				"Length3": PDFNumber(0),
			},
			Data:       font.File,
			Compressed: font.CompressedFile,
		}
		file_ref := doc.TopLevelObject(file)

		// make the font descriptor
		descriptor := PDFMap{
			"Type":     PDFName("FontDescriptor"),
			"FontName": PDFName(font.Name),
			"Flags":    PDFNumber(font.Flags),
			"FontBBox": PDFSlice{
				PDFNumber(font.BBoxLeft),
				PDFNumber(font.BBoxBottom),
				PDFNumber(font.BBoxRight),
				PDFNumber(font.BBoxTop),
			},
			"ItalicAngle": PDFNumber(font.ItalicAngle),
			"Ascent":      PDFNumber(font.Ascent),
			"Descent":     PDFNumber(font.Descent),
			"CapHeight":   PDFNumber(font.CapHeight),
			"StemV":       PDFNumber(font.StemV),
			"FontFile":    file_ref,
		}
		descriptor_ref := doc.TopLevelObject(descriptor)

		fontobject = PDFMap{
			"Type":           PDFName("Font"),
			"Subtype":        PDFName("Type1"),
			"BaseFont":       PDFName(font.Name),
			"FirstChar":      PDFNumber(font.FirstChar),
			"LastChar":       PDFNumber(font.LastChar),
			"Widths":         widths,
			"FontDescriptor": descriptor_ref,
		}
	}

	// does it need an encoding?
	if font.LastChar > 0x7f {
		differences := PDFSlice{PDFNumber(0x80)}
		for i := rune(0x80); i <= font.LastChar; i++ {
			differences = append(differences, PDFName(font.CodePointToName[i]))
		}
		encoding := PDFMap{
			"Type":        PDFName("Encoding"),
			"Differences": differences,
		}
		encoding_ref := doc.TopLevelObject(encoding)
		fontobject["Encoding"] = encoding_ref
	}

	ref = doc.TopLevelObject(fontobject)
	return
}
