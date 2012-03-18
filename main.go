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
)

func main() {
	// first load the fonts
	roman, err := parseFontMetricsFile(filepath.Join(fontPrefix, romanFont))
	if err != nil {
		log.Fatal("loading roman font: ", err)
	}
	roman.Label = "FR"
	bold, err := parseFontMetricsFile(filepath.Join(fontPrefix, boldFont))
	if err != nil {
		log.Fatal("loading bold font: ", err)
	}
	bold.Label = "FB"
	typewriter, err := parseFontMetricsFile(filepath.Join(fontPrefix, typewriterFont))
	if err != nil {
		log.Fatal("loading typewriter font: ", err)
	}
	typewriter.Label = "FT"

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

	fmt.Printf("Font size found: %v\n", dir.FontSize)

	if err = dir.splitIntoLines(); err != nil {
		log.Fatal("splitting families into lines: ", err)
	}

	for _, n := range dir.Columnbreaks {
		fmt.Printf("Column break at entry %v (%v)\n", n, dir.Families[n].Surname)
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

func (dir *Directory) makePDF() error {
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
	doc.AddObject(fmt.Sprintf(obj_font, "/"+dir.Roman.Name))
	doc.AddObject(fmt.Sprintf(obj_font, "/"+dir.Bold.Name))
	doc.AddObject(fmt.Sprintf(obj_font, "/"+dir.Typewriter.Name))
	col := 0
	for page := 0; page < 2; page++ {
		text := dir.Header
		for i := 0; i < dir.ColumnsPerPage; i++ {
			text += dir.Columns[col]
			col++
		}
		doc.AddStream(obj_page_stream, []byte(text))
	}
	doc.WriteTrailer(info, catalog)
	doc.Dump()
	return nil
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
