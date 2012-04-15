package main

import (
	"bytes"
	"code.google.com/p/gorilla/schema"
	"encoding/json"
	"github.com/russross/warddirectory/data"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"unicode/utf8"
)

var t *template.Template
var defaultConfig Directory
var decoder = schema.NewDecoder()
var jquery = MustDecodeBase64(data.Jquery_js)
var favicon = MustDecodeBase64(data.Favicon_ico)

func init() {
	var err error

	// now load the templates
	t = new(template.Template)
	t.Funcs(template.FuncMap{
		"ifEqual": ifEqual,
	})
	template.Must(t.Parse(indexTemplate))

	// load the default config file
	var raw = []byte(data.DefaultConfigJSON)
	if err = json.Unmarshal(raw, &defaultConfig); err != nil {
		log.Fatal("Unable to parse default config file: ", err)
	}
	defaultConfig.ComputeImplicitFields()
	defaultConfig.Roman = FontList["times-roman"]
	defaultConfig.Bold = FontList["times-bold"]

	http.HandleFunc("/", index)
	http.HandleFunc("/submit", submit)
	http.HandleFunc("/jquery.js", js)
	http.HandleFunc("/favicon.ico", ico)
}

func saveLocalConfig(config *Directory) (err error) {
	// where to save?
	where := os.Getenv("USERPROFILE")
	if where == "" {
		where = os.Getenv("HOME")
	}
	if where == "" {
		panic("Unable to find home directory")
	}

	where = filepath.Join(where, ".warddirectory.json")
	data, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		return
	}
	if err = ioutil.WriteFile(where, data, 0644); err != nil {
		return
	}
	return
}

func loadLocalConfig(config *Directory) (err error) {
	// where to load?
	where := os.Getenv("USERPROFILE")
	if where == "" {
		where = os.Getenv("HOME")
	}
	if where == "" {
		panic("Unable to find home directory")
	}

	where = filepath.Join(where, ".warddirectory.json")
	data, err := ioutil.ReadFile(where)

	// ignore file not found
	if err != nil {
		if os.IsNotExist(err) {
			err = nil
		}
		return
	}

	if err = json.Unmarshal(data, config); err != nil {
		return
	}

	return
}

func deleteLocalConfig() (err error) {
	// where to delete?
	where := os.Getenv("USERPROFILE")
	if where == "" {
		where = os.Getenv("HOME")
	}
	if where == "" {
		panic("Unable to find home directory")
	}

	where = filepath.Join(where, ".warddirectory.json")
	err = os.Remove(where)
	if err != nil {
		if os.IsNotExist(err) {
			err = nil
		}
		return
	}
	return
}

// if the first arguments match each other, return the last as a string
func ifEqual(args ...interface{}) string {
	for i := 0; i < len(args)-2; i++ {
		if args[i] != args[i+1] {
			return ""
		}
	}
	if len(args) == 0 {
		return ""
	}
	return args[len(args)-1].(string)
}

func index(w http.ResponseWriter, r *http.Request) {
	// load the user's config data (fonts will not be used)
	config := defaultConfig.Copy()
	err := loadLocalConfig(config)
	if err != nil {
		log.Printf("index: Failure loading config data from disk: %v", err)
		http.Error(w, "Failure loading config data from disk: "+err.Error(),
			http.StatusInternalServerError)
		return
	}

	// append a blank entry to each regexp list
	config.PhoneRegexps = append(config.PhoneRegexps, &RegularExpression{})
	config.AddressRegexps = append(config.AddressRegexps, &RegularExpression{})
	config.NameRegexps = append(config.NameRegexps, &RegularExpression{})

	t.Execute(w, config)
}

func submit(w http.ResponseWriter, r *http.Request) {
	// start with the default config
	config := defaultConfig.Copy()

	// load saved config into it
	err := loadLocalConfig(config)
	if err != nil {
		log.Printf("submit: Failure loading config data from disk: %v", err)
		http.Error(w, "Failure loading config data from disk: "+err.Error(),
			http.StatusInternalServerError)
		return
	}

	// next fill it in using data from the submitted form
	r.ParseMultipartForm(1e6)
	config.Author = "Local clerk"

	// checkboxes are missing if false, so set the checkbox
	// values to false before decoding
	config.FullFamily = false
	config.FamilyPhone = false
	config.FamilyEmail = false
	config.FamilyAddress = false
	config.PersonalPhones = false
	config.PersonalEmails = false

	if err := decoder.Decode(config, r.Form); err != nil {
		log.Printf("Decoding form data: %v", err)
		http.Error(w, "submit: Decoding form data: "+err.Error(), http.StatusBadRequest)
		return
	}

	// set the typewriter font
	if font, present := FontList[config.EmailFont]; present {
		config.Typewriter = font.Copy()
	} else {
		config.Typewriter = FontList[FallbackTypewriter].Copy()
	}
	config.CompileRegexps()
	config.ComputeImplicitFields()

	action := r.FormValue("SubmitButton")

	// almost always save the uploaded form data
	if !strings.HasPrefix(action, "Clear") && !strings.HasPrefix(action, "Import") {
		if err := saveLocalConfig(config); err != nil {
			log.Printf("submit: Saving: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// now figure out what to do with it
	switch {
	case strings.HasPrefix(action, "Clear"):
		if err := deleteLocalConfig(); err != nil {
			log.Printf("Delete: deleting the entry: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/", http.StatusFound)

	case strings.HasPrefix(action, "Export"):
		// convert it into JSON format
		data, err := json.MarshalIndent(config, "", "    ")
		if err != nil {
			log.Printf("Download: json.MarshalIndent: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// return it to the browser
		w.Header()["Content-Type"] = []string{"application/json"}
		w.Header()["Content-Disposition"] =
			[]string{`attachment; filename="WardDirectorySetup.json"`}
		w.Write(data)

	case strings.HasPrefix(action, "Import"):
		// get the uplaoded JSON file
		file, _, err := r.FormFile("DirectoryConfig")
		if err != nil {
			log.Printf("Upload: getting the DirectoryConfig form field: %v", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer file.Close()
		data, err := ioutil.ReadAll(file)
		if err != nil {
			log.Printf("Upload: reading the JSON file data: %v", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// unpack it (font will not be used)
		config := defaultConfig.Copy()
		if err = json.Unmarshal(data, config); err != nil {
			log.Printf("Upload: unable to parse uploaded file: %v", err)
			http.Error(w, "Unable to parse uploaded file: "+err.Error(), http.StatusBadRequest)
			return
		}
		config.ComputeImplicitFields()

		// delete the old one (if any)
		if err := deleteLocalConfig(); err != nil {
			log.Printf("Upload: deleting the old entry: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// store the new one
		if err := saveLocalConfig(config); err != nil {
			log.Printf("Upload: saving the new entry: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/", http.StatusFound)

	case action == "Generate":
		// get the uplaoded CSV data
		file, _, err := r.FormFile("MembershipData")
		if err != nil {
			log.Printf("Generate: getting MembershipData form field: %v", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer file.Close()
		contents, err := ioutil.ReadAll(file)
		if err != nil {
			log.Printf("Generate: reading uploaded file: %v", err)
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
			log.Printf("Generate: parsing families: %v", err)
			http.Error(w, "parsing families: "+err.Error(), http.StatusBadRequest)
			return
		}

		// format families
		config.FormatFamilies()

		// find the font size
		var rounds int
		if rounds, err = config.FindFontSize(); err != nil {
			log.Printf("Generate: finding font site: %v", err)
			http.Error(w, "finding font size: "+err.Error(), http.StatusBadRequest)
			return
		}
		log.Printf("Found font size %.3f in %d rounds", config.FontSize, rounds)

		// render the header and footer
		config.RenderHeader()
		config.RenderFooter()

		// render the family listings
		config.SplitIntoLines()
		config.RenderColumns()

		// generate the PDF file
		var pdf []byte
		if pdf, err = config.MakePDF(); err != nil {
			log.Printf("Generate: making the PDF: %v", err)
			http.Error(w, "making the PDF: "+err.Error(), http.StatusBadRequest)
			return
		}

		// set the headers and send the PDF back to the browser
		w.Header()["Content-Type"] = []string{"application/pdf"}
		w.Header()["Content-Disposition"] =
			[]string{`attachment; filename="directory.pdf"`}
		w.Write(pdf)

	case action == "Shutdown":
		w.Write([]byte("<h1>Goodbye</h1>\n"))
		w.(http.Flusher).Flush()
		log.Fatal("Shutdown at user's request")

	default:
		// save
		http.Redirect(w, r, "/", http.StatusFound)
	}
}

func js(w http.ResponseWriter, r *http.Request) {
	w.Header()["Content-Type"] = []string{"application/javascript"}
	w.Write(jquery)
}

func ico(w http.ResponseWriter, r *http.Request) {
	w.Header()["Content-Type"] = []string{"image/x-icon"}
	w.Write(favicon)
}

func main() {
	log.Print("Ward Directory Generator")
	log.Print("Open a browser and go to http://localhost:1830/")
	log.Print("See http://russross.github.com/warddirectory/ for more information")
	log.Fatal(http.ListenAndServe(":1830", nil))
}
