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
var defaultConfig *Directory
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
	// this is missing from the cmtt font metric file
	typewriter.StemV = typewriterStemV
	if typewriter.File, err = ioutil.ReadFile(filepath.Join(fontPrefix, typewriterFontFile)); err != nil {
		log.Fatal("loading typewriter font: ", err)
	}
	var compressed bytes.Buffer
	var writer *zlib.Writer
	if writer, err = zlib.NewWriterLevel(&compressed, zlib.BestCompression); err != nil {
		panic("Setting up zlib compressor: " + err.Error())
	}
	if _, err = writer.Write(typewriter.File); err != nil {
		panic("Writing to zlib compressor: " + err.Error())
	}
	if err = writer.Close(); err != nil {
		panic("Closing zlib compressor: " + err.Error())
	}
	typewriter.CompressedFile = compressed.Bytes()

	// now load the templates
	t = template.Must(template.ParseFiles("index.template"))

	// load the default config file
	var raw []byte
	if raw, err = ioutil.ReadFile("config.json"); err != nil {
		log.Fatal("loading default config file: ", err)
	}
	defaultConfig = new(Directory)
	if err = json.Unmarshal(raw, defaultConfig); err != nil {
		log.Fatal("Unable to parse default config file: ", err)
	}
	defaultConfig.Prepare()
	defaultConfig.Roman = roman
	defaultConfig.Bold = bold
	defaultConfig.Typewriter = typewriter

	http.HandleFunc("/", index)
	http.HandleFunc("/save", save)
	http.HandleFunc("/generate", generate)
	http.HandleFunc("/upload", upload)
}

func index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Redirect(w, r, "/", http.StatusFound)
	}

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

func save(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	u := user.Current(c)
	if u == nil {
		http.Error(w, "Must be logged in", http.StatusUnauthorized)
		return
	}
	key := datastore.NewKey(c, "Config", u.Email, 0, nil)

	// fill it in using data from the submitted form
	r.ParseForm()
	config := defaultConfig.Copy()
	if err := decoder.Decode(config, r.Form); err != nil {
		http.Error(w, "Decoding form data: "+err.Error(), http.StatusInternalServerError)
		return
	}
	config.CompileRegexps()
	config.ToDatastore()

	// now figure out what to do with it
	switch r.FormValue("SubmitButton") {
	case "Save":
		if _, err := datastore.Put(c, key, config); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/", http.StatusFound)

	case "Download":
		if _, err := datastore.Put(c, key, config); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// convert it into JSON format
		data, err := json.MarshalIndent(config, "", "    ")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// return it to the browser
		w.Header()["Content-Type"] = []string{"application/json"}
		w.Header()["Content-Disposition"] =
			[]string{`attachment; filename="directory_config.json"`}
		w.Write(data)

	case "Delete":
		if err := datastore.Delete(c, key); err != nil && err != datastore.ErrNoSuchEntity {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/", http.StatusFound)
	}
}

func generate(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	u := user.Current(c)
	if u == nil {
		http.Error(w, "Must be logged in", http.StatusUnauthorized)
		return
	}
	key := datastore.NewKey(c, "Config", u.Email, 0, nil)

	// load the user's config data
	dir := defaultConfig.Copy()
	err := datastore.Get(c, key, dir)
	if err == datastore.ErrNoSuchEntity {
		// use defaults
	} else if err != nil {
		http.Error(w, "Failure loading config data from datastore: "+err.Error(),
			http.StatusInternalServerError)
		return
	}
	dir.FromDatastore()
	dir.Prepare()
	dir.CompileRegexps()

	// get the uplaoded CSV data
	file, _, err := r.FormFile("MembershipData")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	// load and parse the families
	if err = dir.ParseFamilies(file); err != nil {
		http.Error(w, "parsing families: "+err.Error(), http.StatusBadRequest)
		return
	}

	// format families
	dir.FormatFamilies()

	// find the font size
	if err = dir.FindFontSize(); err != nil {
		http.Error(w, "finding font size: "+err.Error(), http.StatusBadRequest)
		return
	}

	// render the header
	dir.RenderHeader()

	// render the family listings
	dir.SplitIntoLines()
	dir.RenderColumns()

	// generate the PDF file
	var pdf []byte
	if pdf, err = dir.MakePDF(); err != nil {
		http.Error(w, "making the PDF: "+err.Error(), http.StatusBadRequest)
		return
	}

	// set the headers and send the PDF back to the browser
	w.Header()["Content-Type"] = []string{"application/pdf"}
	w.Header()["Content-Disposition"] =
		[]string{`attachment; filename="directory.pdf"`}
	w.Write(pdf)
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
	config.Prepare()

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
