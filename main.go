package main

import (
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
	romanStemV         = 85
	boldFont           = "ptmb8a.afm"
	boldStemV          = 140
	typewriterFont     = "cmtt10.afm"
	typewriterFontFile = "cmtt10.pfb"
	typewriterStemV    = 125
	ForChurchUseOnly   = "For Church Use Only"
	CompressStreams    = true
	//typewriterFont     = "pcrr8a.afm"
)

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
	title := "Diamond Valley Second Ward"
	dir := NewDirectory(title, roman, bold, typewriter)

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

	if err = dir.splitIntoLines(); err != nil {
		log.Fatal("splitting families into lines: ", err)
	}

	// render the family listings
	if err = dir.renderColumns(); err != nil {
		log.Fatal("rendering columns: ", err)
	}

	// render the header
	if err = dir.renderHeader(); err != nil {
		log.Fatal("rendering header: ", err)
	}

	// generate the PDF file
	if err = dir.makePDF(); err != nil {
		log.Fatal("making the PDF: ", err)
	}
}

func (dir *Directory) makePDF() (err error) {
	// make the PDF file
	doc := NewDocument()

	timestamp := time.Now().Format("20060102150405-0700")
	timestamp = "D:" + timestamp[:17] + "'" + timestamp[17:19] + "'" + timestamp[19:]
	title := "Diamond Valley Second Ward"
	author := "Russ Ross"
	info := doc.AddObject(fmt.Sprintf(obj_info, title, author, timestamp, timestamp))
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
	romanwidths := doc.ForwardRef(i)
	i++
	romandescriptor := doc.ForwardRef(i)
	i++
	romanembedded := ""
	if dir.Roman.Filename != "" {
		romanembedded = doc.ForwardRef(i)
		i++
	}

	bold := doc.ForwardRef(i)
	i++
	boldwidths := doc.ForwardRef(i)
	i++
	bolddescriptor := doc.ForwardRef(i)
	i++
	boldembedded := ""
	if dir.Bold.Filename != "" {
		boldembedded = doc.ForwardRef(i)
		i++
	}

	typewriter := doc.ForwardRef(i)
	i++
	typewriterwidths := doc.ForwardRef(i)
	i++
	typewriterdescriptor := doc.ForwardRef(i)
	i++
	typewriterembedded := ""
	if dir.Typewriter.Filename != "" {
		typewriterembedded = doc.ForwardRef(i)
		i++
	}

	doc.AddObject(fmt.Sprintf(obj_fontresource, roman, bold, typewriter))

	// roman font
	doc.AddObject(makeFont(dir.Roman, romanwidths, romandescriptor))
	doc.AddObject(makeWidths(dir.Roman))
	doc.AddObject(makeFontDescriptor(dir.Roman, romanembedded))
	if dir.Roman.Filename != "" {
		var font []byte
		if font, err = ioutil.ReadFile(dir.Roman.Filename); err != nil {
			return
		}
		doc.AddStream(fmt.Sprintf(obj_font_stream, len(font), 0, 0), font)
	}

	// bold font
	doc.AddObject(makeFont(dir.Bold, boldwidths, bolddescriptor))
	doc.AddObject(makeWidths(dir.Bold))
	doc.AddObject(makeFontDescriptor(dir.Bold, boldembedded))
	if dir.Bold.Filename != "" {
		var font []byte
		if font, err = ioutil.ReadFile(dir.Bold.Filename); err != nil {
			return
		}
		doc.AddStream(fmt.Sprintf(obj_font_stream, len(font), 0, 0), font)
	}

	// typewriter font
	doc.AddObject(makeFont(dir.Typewriter, typewriterwidths, typewriterdescriptor))
	doc.AddObject(makeWidths(dir.Typewriter))
	doc.AddObject(makeFontDescriptor(dir.Typewriter, typewriterembedded))
	if dir.Typewriter.Filename != "" {
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
