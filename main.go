package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

const (
	fontPrefix       = "fonts"
	romanFont        = "ptmr8a.afm"
	boldFont         = "ptmb8a.afm"
	typewriterFont   = "pcrr8a.afm"
	ForChurchUseOnly = "For Church Use Only"
	CompressStreams  = true
)

func main() {
	// first load the fonts
	roman, err := parseFontMetricsFile(filepath.Join(fontPrefix, romanFont), "FR")
	if err != nil {
		log.Fatal("loading roman font: ", err)
	}
	bold, err := parseFontMetricsFile(filepath.Join(fontPrefix, boldFont), "FB")
	if err != nil {
		log.Fatal("loading bold font: ", err)
	}
	typewriter, err := parseFontMetricsFile(filepath.Join(fontPrefix, typewriterFont), "FT")
	if err != nil {
		log.Fatal("loading typewriter font: ", err)
	}

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
	doc.AddObject(fmt.Sprintf(obj_page, pages, fontResource, page1Contents))
	doc.AddObject(fmt.Sprintf(obj_page, pages, fontResource, page2Contents))

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

	// bold font
	doc.AddObject(makeFont(dir.Bold, boldwidths, bolddescriptor))
	doc.AddObject(makeWidths(dir.Bold))
	doc.AddObject(makeFontDescriptor(dir.Bold, boldembedded))

	// typewriter font
	doc.AddObject(makeFont(dir.Typewriter, typewriterwidths, typewriterdescriptor))
	doc.AddObject(makeWidths(dir.Typewriter))
	doc.AddObject(makeFontDescriptor(dir.Typewriter, typewriterembedded))

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

//func testPara() {
//	font, err := parseFontMetricsFile("/usr/share/texmf-texlive/fonts/afm/adobe/times/ptmr8a.afm")
//	if err != nil {
//		fmt.Println(err)
//		return
//	}
//	var words []*Box
//	box, _ := font.MakeBox("Ross")
//	words = append(words, box)
//	box, _ = font.MakeBox("Russ")
//	box.Penalty = 1
//	words = append(words, box)
//	box, _ = font.MakeBox("(773-5952,")
//	box.Penalty = 1
//	words = append(words, box)
//	box, _ = font.MakeBox("russ@russross.com),")
//	words = append(words, box)
//	box, _ = font.MakeBox("Nancy")
//	box.Penalty = 1
//	words = append(words, box)
//	box, _ = font.MakeBox("(773-5953,")
//	box.Penalty = 1
//	words = append(words, box)
//	box, _ = font.MakeBox("nancy@nancyross.com),")
//	words = append(words, box)
//	box, _ = font.MakeBox("Rosie,")
//	words = append(words, box)
//	box, _ = font.MakeBox("Alex,")
//	words = append(words, box)
//	box, _ = font.MakeBox("1414 Agate Ct")
//	words = append(words, box)
//		
//	dir := NewDirectory()
//	fontsize := 10.0
//	firstlinewidth := dir.ColumnWidth / fontsize * 1000.0
//
//	// we'll indent remaining lines using the golden ratio. why? because we can
//	ratio := (1.0 + math.Sqrt(5.0)) / 2.0
//	linewidth := firstlinewidth - fontsize * ratio / fontsize * 1000.0
//	lines := BreakParagraph(words, firstlinewidth, linewidth, float64(font.Glyphs["space"].Width))
//	fmt.Println(lines)
//}
//
//func testFont() {
//	font, err := parseFontMetricsFile("/usr/share/texmf-texlive/fonts/afm/adobe/times/ptmr8a.afm")
//	if err != nil {
//		fmt.Println(err)
//		return
//	}
//	fmt.Printf("Font: %s\n", font.Name)
//	//	for key, val := range font.Glyphs {
//	//		fmt.Printf("%s: %v\n", key, val)
//	//	}
//	box, err := font.MakeBox("Yes, find me a sandwich. ")
//	if err != nil {
//		fmt.Println(err)
//		return
//	}
//	fmt.Printf("%#v\n", box)
//}
//
//func makepage1() string {
//	return `1 0 0 1 36 727 Tm
///FB 20 Tf
//(Ross) Tj
///FR 20 Tf
//( Russ \(773-5952, ) Tj
///FT 20 Tf
//(russ@russross.com) Tj
///FR 20 Tf
//(\),) Tj
//1 0 0 1 52 707 Tm
//[(Nanc)15(y \(773-5953, )] TJ
///FT 20 Tf
//(nancy@nancyross.com) Tj
///FR 20 Tf
//(\), Rosie, Alex,) Tj
//1 0 0 1 52 687 Tm
//(1414 Agate Ct) Tj
//1 0 0 1 52 667 Tm
//[(Y)100(es, )<ae>(nd me a sandwich.)] TJ
//1 0 0 1 251.08 667 Tm
//([End])Tj
//`
//}
