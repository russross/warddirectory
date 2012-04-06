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
	Phone   string
	Email   string
	People  []*Person
}

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

func prepName(regexps []*RegularExpression, name string) string {
	// prepare name
	for _, re := range regexps {
		name = re.Regexp.ReplaceAllString(strings.TrimSpace(name), re.Replacement)
		name = Spaces.ReplaceAllString(name, " ")
		name = strings.TrimSpace(name)
	}

	return name
}

func prepAddress(regexps []*RegularExpression, address string) string {
	// prepare address
	for _, re := range regexps {
		address = re.Regexp.ReplaceAllString(strings.TrimSpace(address), re.Replacement)
		address = Spaces.ReplaceAllString(address, " ")
		address = strings.TrimSpace(address)
	}

	return address
}

var Phone10Digit = regexp.MustCompile(`^\D*(\d{3})\D*(\d{3})\D*(\d{4})\D*$`)
var Phone7Digit = regexp.MustCompile(`^\D*(\d{3})\D*(\d{4})\D*$`)

// prepare phone number
func prepPhone(regexps []*RegularExpression, phone, familyPhone string) string {
	// first extract groups of digits and put it in the form 123-456-7890
	phone = Phone10Digit.ReplaceAllString(phone, "$1-$2-$3")

	// same for 123-4567
	phone = Phone7Digit.ReplaceAllString(phone, "$1-$2")

	for _, re := range regexps {
		phone = re.Regexp.ReplaceAllString(phone, re.Replacement)
		phone = Spaces.ReplaceAllString(phone, " ")
		phone = strings.TrimSpace(phone)
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

var Spaces = regexp.MustCompile(`\s+`)

func (dir *Directory) ParseFamilies(src io.Reader) error {
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
			elt = Spaces.ReplaceAllString(elt, " ")
			fields[i] = strings.TrimSpace(elt)
		}

		// gather info that is the same for the entire family
		family := new(Family)
		family.Surname, family.Couple, family.Phone, family.Email, family.Address =
			fields[0], fields[1], fields[2], fields[3], fields[4]

		// gather the individual family members
		var familyMembers [][]string
		end := len(fields)
		if !dir.FullFamily && end > 11 {
			end = 11
		}

		for i := 5; i < end; i += 3 {
			familyMembers = append(familyMembers, fields[i:i+3])
		}

		// prepare couple name
		family.Couple = ""

		// prepare address
		if dir.FamilyAddress {
			family.Address = prepAddress(dir.AddressRegexps, family.Address)
		} else {
			family.Address = ""
		}

		// prepare the family phone number
		if dir.FamilyPhone {
			family.Phone = prepPhone(dir.PhoneRegexps, family.Phone, "")
		} else {
			family.Phone = ""
		}

		// prepare family email address
		if dir.FamilyEmail {
			family.Email = prepEmail(family.Email, "")
		} else {
			family.Email = ""
		}

		// gather the list of family members
		for _, individual := range familyMembers {
			person := new(Person)
			person.Name, person.Phone, person.Email = individual[0], individual[1], individual[2]

			// empty entry?
			if person.Name == "" {
				continue
			}

			// prepare name
			person.Name = prepName(dir.NameRegexps, person.Name)

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
			if dir.PersonalPhones {
				person.Phone = prepPhone(dir.PhoneRegexps, person.Phone, family.Phone)
			} else {
				person.Phone = ""
			}

			// prepare individual email address
			if dir.PersonalEmails {
				person.Email = prepEmail(person.Email, family.Email)
			} else {
				person.Email = ""
			}

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
func packBox(lst []*Box, elt string, space int, font *FontMetrics) (entry []*Box) {
	var box *Box
	if len(lst) == 0 {
		return []*Box{font.MakeBox(elt, 1.0)}
	}

	prev := lst[len(lst)-1]

	switch {
	// can we tack this on to the end of the previous box?
	case space < 0 && prev.Font == font:
		box = font.MakeBox(prev.Original+elt, 1.0)
		lst[len(lst)-1] = box
		return lst

	// join this to the previous box, but with different fonts
	case space < 0:
		prev.JoinNext = true
		box = font.MakeBox(elt, 1.0)
		return append(lst, box)

	// make a new box
	default:
		box = font.MakeBox(elt, 1.0)
		prev.Penalty = space
		return append(lst, box)
	}

	panic("Can't get here")
}

func (dir *Directory) FormatFamilies() {
	for _, family := range dir.Families {
		var entry []*Box

		// start with the surname in bold
		for i, word := range strings.Fields(family.Surname) {
			space := 0

			// strongly discourage line breaks within a surname
			if i > 0 {
				space = 2
			}
			entry = packBox(entry, word, space, dir.Bold)
		}

		needcomma := false

		// next the couple name (if that is all that was requested)
		if family.Couple != "" {
			if needcomma {
				entry = packBox(entry, ",", -1, dir.Roman)
				needcomma = false
			}
			for i, name := range strings.Split(family.Couple, " & ") {
				space := 0

				// discourage line breaks between people
				if i > 0 {
					entry = packBox(entry, "&", 2, dir.Roman)
					space = 1
				}

				// split the person's name into discrete words
				for j, word := range strings.Fields(name) {
					// strongly discourage line breaks within a person's name
					if j > 0 {
						space = 2
					}
					entry = packBox(entry, word, space, dir.Roman)
				}
			}
			needcomma = true
		}

		// next the phone number (if present)
		if family.Phone != "" {
			if needcomma {
				entry = packBox(entry, ",", -1, dir.Roman)
				needcomma = false
			}
			entry = packBox(entry, family.Phone, 0, dir.Roman)
			needcomma = true
		}

		// next the email address (if present)
		if family.Email != "" {
			if needcomma {
				entry = packBox(entry, ",", -1, dir.Roman)
				needcomma = false
			}
			entry = packBox(entry, family.Email, 0, dir.Typewriter)
			needcomma = true
		}

		// now the family members
		for _, person := range family.People {
			if needcomma {
				entry = packBox(entry, ",", -1, dir.Roman)
				needcomma = false
			}

			// split the person's name into discrete words
			for i, word := range strings.Fields(person.Name) {
				space := 0

				// strongly discourage line breaks within a person's name
				if i > 0 {
					space = 2
				}
				entry = packBox(entry, word, space, dir.Roman)
			}

			// no contact details?  just end with a comma
			if person.Phone == "" && person.Email == "" {
				needcomma = true
				continue
			}

			// phone and email address are in parentheses
			entry = packBox(entry, "(", 1, dir.Roman)

			// phone
			if person.Phone != "" {
				entry = packBox(entry, person.Phone, -1, dir.Roman)
				needcomma = true
			}

			// email
			if person.Email != "" {
				// discourage line breaks within a person's entry
				space := 1

				if needcomma {
					entry = packBox(entry, ",", -1, dir.Roman)
					needcomma = false
				} else {
					space = -1
				}

				// the email address
				entry = packBox(entry, person.Email, space, dir.Typewriter)
			}

			// close paren and comma
			entry = packBox(entry, ")", -1, dir.Roman)
			needcomma = true
		}

		// address comes next
		// split the address into words
		if family.Address != "" {
			if needcomma {
				entry = packBox(entry, ",", -1, dir.Roman)
				needcomma = false
			}

			words := strings.Fields(family.Address)
			for i, word := range words {
				space := 0

				// strongly discourage line breaks within an address
				// (but not after a comma)
				if i > 0 && !strings.HasSuffix(words[i-1], ",") {
					space = 2
				}

				entry = packBox(entry, word, space, dir.Roman)
			}
			needcomma = true
		}

		dir.Entries = append(dir.Entries, entry)
	}
}

var FallbackRegexp = regexp.MustCompile(`^I don't match anything$`)

func (dir *Directory) CompileRegexps() {
	var err error
	phone := dir.PhoneRegexps
	dir.PhoneRegexps = nil
	for _, elt := range phone {
		if strings.TrimSpace(elt.Expression) != "" {
			dir.PhoneRegexps = append(dir.PhoneRegexps, elt)
		}
		if elt.Regexp, err = regexp.Compile("(?i:" + elt.Expression + ")"); err != nil {
			elt.Regexp = FallbackRegexp
			elt.Expression = "!!Error!! " + elt.Expression
		}
	}
	address := dir.AddressRegexps
	dir.AddressRegexps = nil
	for _, elt := range address {
		if strings.TrimSpace(elt.Expression) != "" {
			dir.AddressRegexps = append(dir.AddressRegexps, elt)
		}
		if elt.Regexp, err = regexp.Compile("(?i:" + elt.Expression + ")"); err != nil {
			elt.Regexp = FallbackRegexp
			elt.Expression = "!!Error!! " + elt.Expression
		}
	}
	name := dir.NameRegexps
	dir.NameRegexps = nil
	for _, elt := range name {
		if strings.TrimSpace(elt.Expression) != "" {
			dir.NameRegexps = append(dir.NameRegexps, elt)
		}
		if elt.Regexp, err = regexp.Compile("(?i:" + elt.Expression + ")"); err != nil {
			elt.Regexp = FallbackRegexp
			elt.Expression = "!!Error!! " + elt.Expression
		}
	}

	return
}
