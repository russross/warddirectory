//
// Make a printable ward directory
// from the data downloaded from the lds.org directory
//
// Input is from stdin, output goes to stdout
//
// This script produces a list of families,
// which is embedded in the directory.tex LaTeX file.
// The accompanying resize.py script finds the right
// font size to squeeze it all onto a single 2-sided sheet.
//
// by Russ Ross <russ@russross.com>
// idea and formatting details taken from
// similar work by Richard Ross
//

package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
)

type Person struct {
	Name  string
	Phone string
	Email string
}

type Family struct {
	Surname string
	Couple  string
	Address string
	City    string
	State   string
	Zip     string
	Phone   string
	Email   string
	People  []*Person
}

var (
	AREACODE = "435"
	CITY     = "Diamond Valley"
	STATE    = "Utah"
	CITIES   = []string{"Diamond Valley", "Dammeron Valley", "St. George"}
	STATES   = []string{"Utah"}
)

// the regexp to split an address into address, city, state components
var ADDRESS_RE = regexp.MustCompile(
	`^(.*?)\s*(` +
		strings.Join(CITIES, "|") +
		`)?,\s*(` +
		strings.Join(STATES, "|") +
		`)?\s*([\d-]+)?$`)

// list of uniform-length slices
type familyList []*Family

func (lst familyList) Len() int {
	return len(lst)
}

func (lst familyList) Less(i, j int) bool {
	if lst[i].Surname < lst[j].Surname {
		return true
	}
	if lst[i].Couple < lst[i].Couple {
		return true
	}
	return false
}

func (lst familyList) Swap(i, j int) {
	lst[i], lst[j] = lst[j], lst[i]
}

var headerFields = []string{
	"Family Name", "Couple Name",
	"Family Phone", "Family Email", "Family Address",
	"Head Of House Name", "Head Of House Phone", "Head Of House Email",
	"Spouse Name", "Spouse Phone", "Spouse Email",
	"Child Name", "Child Phone", "Child Email",
	"Child Name", "Child Phone", "Child Email",
	"Child Name", "Child Phone", "Child Email",
	"Child Name", "Child Phone", "Child Email",
	"Child Name", "Child Phone", "Child Email",
	"Child Name", "Child Phone", "Child Email",
	"Child Name", "Child Phone", "Child Email",
	"Child Name", "Child Phone", "Child Email",
	"Child Name", "Child Phone", "Child Email",
	"Child Name", "Child Phone", "Child Email",
	"Child Name", "Child Phone", "Child Email",
	"Child Name", "Child Phone", "Child Email",
	"Child Name", "Child Phone", "Child Email",
	"Child Name", "Child Phone", "Child Email",
	"Child Name", "Child Phone", "Child Email",
	"Child Name", "Child Phone", "Child Email",
}

func prepStreet(street string) string {
	// insert cleanups and abbreviations here

	// escape # for LaTeX
	street = strings.Replace(street, "#", "\\#", -1)

	return street
}

var phone10digit = regexp.MustCompile(`^\D*(\d{3})\D*(\d{3})\D*(\d{4})\D*$`)
var phone7digit = regexp.MustCompile(`^\D*(\d{3})\D*(\d{4})\D*$`)

func prepPhone(phone, familyPhone string) string {
	// prepare phone number
	// look for groups of digits, ignore everything else
	if parts := phone10digit.FindStringSubmatch(phone); len(parts) == 4 {
		phone = strings.Join(parts[1:], "-")
	} else if parts := phone7digit.FindStringSubmatch(phone); len(parts) == 3 {
		phone = strings.Join(parts[1:], "-")
	}

	// string the area code if it is the default
	if len(phone) >= 12 && strings.HasPrefix(phone, AREACODE+"-") {
		phone = phone[4:]
	}

	if phone == familyPhone {
		phone = ""
	}

	return phone
}

func prepEmail(email, familyEmail string) string {
	// escape _ for LaTeX
	email = strings.Replace(email, "_", "\\_", -1)
	if email != "" {
		email = "\\texttt{" + email + "}"
	}

	if email == familyEmail {
		email = ""
	}

	return email
}

func (dir *Directory) parseFamilies(src io.Reader) error {
	reader := csv.NewReader(src)

	// the CSV reader is picky about the number of fields being consistent
	// this relaxes it
	reader.FieldsPerRecord = -1

	// read the header line
	fields, err := reader.Read()
	if err != nil {
		return err
	}

	// verify the header fields are as expected
	if len(fields) != len(headerFields) {
		return fmt.Errorf("Wrong number of header fields; has the file format changed?")
	}
	for i := 0; i < len(fields); i++ {
		if fields[i] != headerFields[i] {
			return fmt.Errorf("Header field mismatch. Expected %s, found %s",
				headerFields[i], fields[i])
		}
	}

	// process one family at a time
	var families familyList
	reader.TrailingComma = true
	for fields, err = reader.Read(); err == nil; fields, err = reader.Read() {
		for i, elt := range fields {
			fields[i] = strings.TrimSpace(elt)
		}

		// gather info that is the same for the entire family
		family := new(Family)
		family.Surname, family.Couple, family.Phone, family.Email, family.Address =
			fields[0], fields[1], fields[2], fields[3], fields[4]

		// gather the individual family members
		var familyMembers [][]string
		for i := 5; i < len(fields); i += 3 {
			familyMembers = append(familyMembers, fields[i:i+3])
		}

		// split the address into street, state, postal
		parts := ADDRESS_RE.FindStringSubmatch(family.Address)
		if parts == nil {
			return fmt.Errorf("Malformed address for %s %s family", family.Couple, family.Surname)
		}
		for i, elt := range parts {
			parts[i] = strings.TrimSpace(elt)
		}
		// street, city, state, postal code
		family.Address, family.City, family.State, family.Zip = parts[1], parts[2], parts[3], parts[4]

		// prepare address
		family.Address = prepStreet(family.Address)

		// prepare the family phone number
		family.Phone = prepPhone(family.Phone, "")

		// prepare family email address
		family.Email = prepEmail(family.Email, "")

		// gather the list of family members
		for _, individual := range familyMembers {
			person := new(Person)
			person.Name, person.Phone, person.Email = individual[0], individual[1], individual[2]

			// empty entry?
			if person.Name == "" {
				continue
			}

			// only show surname if different from family name
			if strings.HasPrefix(strings.ToLower(person.Name), strings.ToLower(family.Surname)+", ") {
				person.Name = person.Name[len(family.Surname)+2:]

				// rearrange as "first last"
				// note: if last name has already been removed, we
				// assume this is "bob, jr" or the like and leave it alone, hence else if
			} else if comma := strings.Index(person.Name, ", "); comma >= 0 {
				person.Name = person.Name[comma+2:] + " " + person.Name[:comma]
			}

			// prepare individual phone number
			person.Phone = prepPhone(person.Phone, family.Phone)

			// prepare individual email address
			person.Email = prepEmail(person.Email, family.Email)

			family.People = append(family.People, person)
		}

		families = append(families, family)
	}
	if err != io.EOF {
		return err
	}

	sort.Sort(families)

	dir.Families = families
	return nil
}

func (dir *Directory) formatFamilies() error {
	for _, family := range dir.Families {
		var entry []*Box

		// start with the surname in bold
		surname, err := dir.Bold.MakeBox(family.Surname)
		if err != nil {
			return err
		}
		entry = append(entry, surname)

		// next the phone number (if present)
		if family.Phone != "" {
			phone, err := dir.Roman.MakeBox(family.Phone + ",")
			if err != nil {
				return err
			}
			entry = append(entry, phone)
		}

		// next the email address (if present)
		if family.Email != "" {
			email, err := dir.Typewriter.MakeBox(family.Email)
			if err != nil {
				return err
			}
			email.JoinNext = true
			entry = append(entry, email)

			comma, err := dir.Roman.MakeBox(",")
			if err != nil {
				return err
			}
			entry = append(entry, comma)
		}

		// now the family members
		for _, person := range family.People {
			// split the person's name into discrete words
			parts := strings.Fields(person.Name)

			for i, word := range parts {
				box, err := dir.Roman.MakeBox(word)
				if err != nil {
					return err
				}

				switch {
				// strongly discourage line breaks within a person's name
				case i+1 < len(parts):
					box.Penalty = 2

				// discourage line breaks between a name and contact info
				case person.Phone != "" || person.Email != "":
					box.Penalty = 1

				// no line break before trailing comma when there is no contact info
				default:
					box.JoinNext = true
				}
				entry = append(entry, box)
			}

			// now take care of phone and email address
			if person.Phone != "" || person.Email != "" {
				open, err := dir.Roman.MakeBox("(")
				if err != nil {
					return err
				}
				open.JoinNext = true
				entry = append(entry, open)

				// phone
				if person.Phone != "" {
					phone, err := dir.Roman.MakeBox(person.Phone)
					if err != nil {
						return err
					}

					// either a comma or a closing paren will follow
					phone.JoinNext = true
					entry = append(entry, phone)

					// comma between phone and email?
					if person.Email != "" {
						comma, err := dir.Roman.MakeBox(",")
						if err != nil {
							return err
						}
						// discourage line breaks within a person's entry
						comma.Penalty = 1
						entry = append(entry, comma)
					}
				}

				if person.Email != "" {
					email, err := dir.Typewriter.MakeBox(person.Email)
					if err != nil {
						return err
					}

					// a closing paren will follow
					email.JoinNext = true
					entry = append(entry, email)
				}

				closep, err := dir.Roman.MakeBox(")")
				if err != nil {
					return err
				}
				closep.JoinNext = true
				entry = append(entry, closep)
			}

			comma, err := dir.Roman.MakeBox(",")
			if err != nil {
				return err
			}
			entry = append(entry, comma)
		}
	}

	return nil
}
