package main

import (
	"appengine"
	"appengine/datastore"
	"appengine/user"
	"bytes"
	"code.google.com/p/gorilla/schema"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"text/template"
	"unicode/utf8"
)

var t *template.Template
var defaultConfig Directory
var decoder = schema.NewDecoder()

func init() {
	var err error

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
	defaultConfig.Typewriter = lmvtt

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
	config.Author = u.Email

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
		contents, err := ioutil.ReadAll(file)
		if err != nil {
			http.Error(w, "reading uploaded file: "+err.Error(), http.StatusBadRequest)
			return
		}
		var src io.Reader

		// see if it is valid utf-8 input
		if !utf8.Valid(contents) {
			// if not, assume it is latin1/windows1252/iss8859-1 and convert it
			runes := make([]rune, len(contents))
			for i, ch := range contents {
				runes[i] = rune(ch)
			}
			src = strings.NewReader(string(runes))
		} else {
			src = bytes.NewReader(contents)
		}

		// load and parse the families
		if err = config.ParseFamilies(src); err != nil {
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
