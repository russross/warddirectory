package main

import (
	"appengine"
	"appengine/datastore"
	"appengine/user"
	"code.google.com/p/gorilla/schema"
	"encoding/json"
	//"html"
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
	if roman, err = ParseFontMetricsFile(filepath.Join(fontPrefix, romanFont), "FR", romanStemV); err != nil {
		log.Fatal("loading roman font metrics: ", err)
	}
	if bold, err = ParseFontMetricsFile(filepath.Join(fontPrefix, boldFont), "FB", boldStemV); err != nil {
		log.Fatal("loading bold font metrics: ", err)
	}
	if typewriter, err = ParseFontMetricsFile(filepath.Join(fontPrefix, typewriterFont), "FT", typewriterStemV); err != nil {
		log.Fatal("loading typewriter font metrics: ", err)
	}
	if typewriter.File, err = ioutil.ReadFile(filepath.Join(fontPrefix, typewriterFontFile)); err != nil {
		log.Fatal("loading typewriter font: ", err)
	}

	// now load the templates
	t = new(template.Template)
	template.Must(t.ParseGlob("*.template"))

	// load the default config file
	var raw []byte
	raw, err = ioutil.ReadFile("config.json")
	if err != nil {
		log.Fatal("loading default config file: ", err)
	}
	defaultConfig, err = NewDirectory(raw, roman, bold, typewriter)
	if err != nil {
		log.Fatal("parsing default config file: ", err)
	}

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
	config := new(Directory)
	err := datastore.Get(c, key, config)
	if err == datastore.ErrNoSuchEntity {
		// use default values
		config = defaultConfig
	} else if err != nil {
		http.Error(w, "Failure loading config data from datastore: "+err.Error(),
			http.StatusInternalServerError)
		return
	} else {
		config.FromDatastore()
	}

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
	config := new(Directory)
	r.ParseForm()
	if err := decoder.Decode(config, r.Form); err != nil {
		http.Error(w, "Decoding form data: "+err.Error(), http.StatusInternalServerError)
		return
	}
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
	dir := new(Directory)
	err := datastore.Get(c, key, dir)
	if err == datastore.ErrNoSuchEntity {
		http.Error(w, "Must save configuration first", http.StatusBadRequest)
		return
	} else if err != nil {
		http.Error(w, "Failure loading config data from datastore: "+err.Error(),
			http.StatusInternalServerError)
		return
	}
	dir.FromDatastore()
	dir.Prepare(roman, bold, typewriter)
	if err = dir.CompileRegexps(); err != nil {
		http.Error(w, "Error in regular expressions: "+err.Error(), http.StatusInternalServerError)
		return
	}

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
	if err = dir.FormatFamilies(); err != nil {
		http.Error(w, "formatting families: "+err.Error(), http.StatusBadRequest)
		return
	}

	// find the font size
	if err = dir.FindFontSize(); err != nil {
		http.Error(w, "finding font size: "+err.Error(), http.StatusBadRequest)
		return
	}

	// render the header
	if err = dir.RenderHeader(); err != nil {
		http.Error(w, "rendering header: "+err.Error(), http.StatusBadRequest)
		return
	}

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
	config, err := NewDirectory(data, roman, bold, typewriter)
	if err != nil {
		http.Error(w, "Unable to parse uploaded file: "+err.Error(), http.StatusBadRequest)
		return
	}

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
