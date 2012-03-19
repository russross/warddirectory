//
// Make a printable ward directory
// from the data downloaded from the lds.org directory
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
	"log"
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

// sortable list
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
			log.Printf("Malformed address for %s family", family.Couple)
		} else {
			for i, elt := range parts {
				parts[i] = strings.TrimSpace(elt)
			}
			// street, city, state, postal code
			family.Address, family.City, family.State, family.Zip = parts[1], parts[2], parts[3], parts[4]

			// prepare address
			family.Address = prepStreet(family.Address)
		}

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

// space: -1 means no leading space 0 regular, 1+ penalty for line break
func packBox(lst []*Box, elt string, space int, font *FontMetrics) (entry []*Box, err error) {
	var box *Box
	if len(lst) == 0 {
		if box, err = font.MakeBox(elt, 1.0); err != nil {
			return nil, err
		}
		return []*Box{box}, nil
	}

	prev := lst[len(lst)-1]

	switch {
	// can we tack this on to the end of the previous box?
	case space < 0 && prev.Font == font:
		if box, err = font.MakeBox(prev.Original+elt, 1.0); err != nil {
			return nil, err
		}
		lst[len(lst)-1] = box
		return lst, nil

	// join this to the previous box, but with different fonts
	case space < 0:
		prev.JoinNext = true
		if box, err = font.MakeBox(elt, 1.0); err != nil {
			return nil, err
		}
		return append(lst, box), nil

	// make a new box
	default:
		if box, err = font.MakeBox(elt, 1.0); err != nil {
			return nil, err
		}
		prev.Penalty = space
		return append(lst, box), nil
	}

	panic("Can't get here")
}

func (dir *Directory) formatFamilies() (err error) {
	for _, family := range dir.Families {
		var entry []*Box

		// start with the surname in bold
		for i, word := range strings.Fields(family.Surname) {
			space := 0

			// strongly discourage line breaks within a surname
			if i > 0 {
				space = 2
			}
			if entry, err = packBox(entry, word, space, dir.Bold); err != nil {
				return
			}
		}

		needcomma := false

		// next the phone number (if present)
		if family.Phone != "" {
			if needcomma {
				if entry, err = packBox(entry, ",", -1, dir.Roman); err != nil {
					return
				}
				needcomma = false
			}
			if entry, err = packBox(entry, family.Phone, 0, dir.Roman); err != nil {
				return
			}
			needcomma = true
		}

		// next the email address (if present)
		if family.Email != "" {
			if needcomma {
				if entry, err = packBox(entry, ",", -1, dir.Roman); err != nil {
					return
				}
				needcomma = false
			}
			if entry, err = packBox(entry, family.Email, 0, dir.Typewriter); err != nil {
				return
			}
			needcomma = true
		}

		// now the family members
		for _, person := range family.People {
			if needcomma {
				if entry, err = packBox(entry, ",", -1, dir.Roman); err != nil {
					return
				}
				needcomma = false
			}

			// split the person's name into discrete words
			for i, word := range strings.Fields(person.Name) {
				space := 0

				// strongly discourage line breaks within a person's name
				if i > 0 {
					space = 2
				}
				if entry, err = packBox(entry, word, space, dir.Roman); err != nil {
					return
				}
			}

			// no contact details?  just end with a comma
			if person.Phone == "" && person.Email == "" {
				needcomma = true
				continue
			}

			// phone and email address are in parentheses
			if entry, err = packBox(entry, "(", 1, dir.Roman); err != nil {
				return
			}

			// phone
			if person.Phone != "" {
				if entry, err = packBox(entry, person.Phone, -1, dir.Roman); err != nil {
					return
				}
				needcomma = true
			}

			// email
			if person.Email != "" {
				// discourage line breaks within a person's entry
				space := 1

				if needcomma {
					if entry, err = packBox(entry, ",", -1, dir.Roman); err != nil {
						return
					}
					needcomma = false
				} else {
					space = -1
				}

				// the email address
				if entry, err = packBox(entry, person.Email, space, dir.Typewriter); err != nil {
					return
				}
			}

			// close paren and comma
			if entry, err = packBox(entry, ")", -1, dir.Roman); err != nil {
				return
			}
			needcomma = true
		}

		// address comes next
		// split the address into words
		if family.Address != "" {
			if needcomma {
				if entry, err = packBox(entry, ",", -1, dir.Roman); err != nil {
					return
				}
				needcomma = false
			}

			for i, word := range strings.Fields(family.Address) {
				space := 0

				// strongly discourage line breaks within an address
				if i > 0 {
					space = 2
				}
				if entry, err = packBox(entry, word, space, dir.Roman); err != nil {
					return
				}
			}
			needcomma = true
		}

		// only show the city if it is not the default
		if family.City != CITY && family.City != "" {
			if needcomma {
				if entry, err = packBox(entry, ",", -1, dir.Roman); err != nil {
					return
				}
				needcomma = false
			}

			// split the city into words
			for i, word := range strings.Fields(family.City) {
				space := 0

				// strongly discourage line breaks within a city
				if i > 0 {
					space = 2
				}
				if entry, err = packBox(entry, word, space, dir.Roman); err != nil {
					return
				}
			}
		}

		dir.Entries = append(dir.Entries, entry)
	}

	return nil
}
