package main

import (
	"appengine"
	"appengine/datastore"
	"appengine/user"
	"bytes"
	"code.google.com/p/gorilla/schema"
	"compress/zlib"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"
	"text/template"
)

var t *template.Template
var roman, bold, typewriter *FontMetrics
var unicodeToGlyph map[rune]string
var defaultConfig Directory
var decoder = schema.NewDecoder()

func init() {
	var err error

	// first load the fonts
	if roman, err = ParseFontMetricsFile(filepath.Join(fontPrefix, romanFont), "FR"); err != nil {
		log.Fatal("loading roman font metrics: ", err)
	}
	if bold, err = ParseFontMetricsFile(filepath.Join(fontPrefix, boldFont), "FB"); err != nil {
		log.Fatal("loading bold font metrics: ", err)
	}
	if typewriter, err = ParseFontMetricsFile(filepath.Join(fontPrefix, typewriterFont), "FT"); err != nil {
		log.Fatal("loading typewriter font metrics: ", err)
	}

	// get the complete list of glyphs we know about
	if unicodeToGlyph, err = GlyphMapping(roman, bold, typewriter, filepath.Join(fontPrefix, glyphlistFile)); err != nil {
		log.Fatal("loading glyph metrics: ", err)
	}

	// this is missing from the cmtt font metric file
	typewriter.StemV = typewriterStemV
	if typewriter.File, err = ioutil.ReadFile(filepath.Join(fontPrefix, typewriterFontFile)); err != nil {
		log.Fatal("loading typewriter font: ", err)
	}
	var compressed bytes.Buffer
	var writer *zlib.Writer
	if writer, err = zlib.NewWriterLevel(&compressed, zlib.BestCompression); err != nil {
		log.Fatal("Setting up zlib compressor: ", err)
	}
	if _, err = writer.Write(typewriter.File); err != nil {
		log.Fatal("Writing to zlib compressor: ", err)
	}
	if err = writer.Close(); err != nil {
		log.Fatal("Closing zlib compressor: ", err)
	}
	typewriter.CompressedFile = compressed.Bytes()

	// now load the templates
	t = template.Must(template.ParseFiles("index.template"))

	// load the default config file
	var raw []byte
	if raw, err = ioutil.ReadFile("config.json"); err != nil {
		log.Fatal("loading default config file: ", err)
	}
	if err = json.Unmarshal(raw, &defaultConfig); err != nil {
		log.Fatal("Unable to parse default config file: ", err)
	}
	defaultConfig.ComputeImplicitFields()
	defaultConfig.Roman = roman
	defaultConfig.Bold = bold
	defaultConfig.Typewriter = typewriter

	http.HandleFunc("/", index)
	http.HandleFunc("/submit", submit)
	http.HandleFunc("/upload", upload)
}

func index(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	u := user.Current(c)
	if u == nil {
		http.Error(w, "Must be logged in", http.StatusUnauthorized)
		return
	}
	key := datastore.NewKey(c, "Config", u.Email, 0, nil)

	// load the user's config data
	config := defaultConfig.Copy()
	err := datastore.Get(c, key, config)
	if err == datastore.ErrNoSuchEntity {
		// use default values
	} else if err != nil {
		http.Error(w, "Failure loading config data from datastore: "+err.Error(),
			http.StatusInternalServerError)
		return
	} else {
		config.FromDatastore()
	}

	// append a blank phone regexp and a blank address regexp
	config.PhoneRegexps = append(config.PhoneRegexps, &RegularExpression{})
	config.AddressRegexps = append(config.AddressRegexps, &RegularExpression{})

	tmpl := t.Lookup("index.template")
	tmpl.Execute(w, config)
}

func submit(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	u := user.Current(c)
	if u == nil {
		http.Error(w, "Must be logged in", http.StatusUnauthorized)
		return
	}
	key := datastore.NewKey(c, "Config", u.Email, 0, nil)

	// fill it in using data from the submitted form
	r.ParseMultipartForm(1e6)
	config := defaultConfig.Copy()

	// checkboxes are missing if false, so set the checkbox
	// values to false before decoding
	config.FullFamily = false
	config.FamilyPhone = false
	config.FamilyEmail = false
	config.FamilyAddress = false
	config.PersonalPhones = false
	config.PersonalEmails = false

	if err := decoder.Decode(config, r.Form); err != nil {
		http.Error(w, "Decoding form data: "+err.Error(), http.StatusBadRequest)
		return
	}
	config.CompileRegexps()
	config.ComputeImplicitFields()
	config.ToDatastore()

	action := r.FormValue("SubmitButton")

	// almost always save the uploaded form data
	if action != "Delete" {
		if _, err := datastore.Put(c, key, config); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// now figure out what to do with it
	switch action {
	case "Delete":
		if err := datastore.Delete(c, key); err != nil && err != datastore.ErrNoSuchEntity {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/", http.StatusFound)

	case "Download":
		// convert it into JSON format
		data, err := json.MarshalIndent(config, "", "    ")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// return it to the browser
		w.Header()["Content-Type"] = []string{"application/json"}
		w.Header()["Content-Disposition"] =
			[]string{`attachment; filename="WardDirectorySetup.json"`}
		w.Write(data)

	case "Generate":
		// get the uplaoded CSV data
		file, _, err := r.FormFile("MembershipData")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer file.Close()

		// load and parse the families
		if err = config.ParseFamilies(file); err != nil {
			http.Error(w, "parsing families: "+err.Error(), http.StatusBadRequest)
			return
		}

		// format families
		config.FormatFamilies()

		// find the font size
		var rounds int
		if rounds, err = config.FindFontSize(); err != nil {
			http.Error(w, "finding font size: "+err.Error(), http.StatusBadRequest)
			return
		}
		c.Infof("Found font size %.3f in %d rounds", config.FontSize, rounds)

		// render the header
		config.RenderHeader()

		// render the family listings
		config.SplitIntoLines()
		config.RenderColumns()

		// generate the PDF file
		var pdf []byte
		if pdf, err = config.MakePDF(); err != nil {
			http.Error(w, "making the PDF: "+err.Error(), http.StatusBadRequest)
			return
		}

		// set the headers and send the PDF back to the browser
		w.Header()["Content-Type"] = []string{"application/pdf"}
		w.Header()["Content-Disposition"] =
			[]string{`attachment; filename="directory.pdf"`}
		w.Write(pdf)

	default:
		// save
		http.Redirect(w, r, "/", http.StatusFound)
	}
}

func upload(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	u := user.Current(c)
	if u == nil {
		http.Error(w, "Must be logged in", http.StatusUnauthorized)
		return
	}
	key := datastore.NewKey(c, "Config", u.Email, 0, nil)

	// get the uplaoded JSON file
	file, _, err := r.FormFile("DirectoryConfig")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()
	data, err := ioutil.ReadAll(file)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// unpack it
	config := defaultConfig.Copy()
	if err = json.Unmarshal(data, config); err != nil {
		http.Error(w, "Unable to parse uploaded file: "+err.Error(), http.StatusBadRequest)
		return
	}
	config.ComputeImplicitFields()

	// delete the old one (if any)
	if err := datastore.Delete(c, key); err != nil && err != datastore.ErrNoSuchEntity {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// store the new one
	config.ToDatastore()
	if _, err := datastore.Put(c, key, config); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
}
