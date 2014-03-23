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
	"regexp"
	"strings"
)

type Person struct {
	Name  string
	Phone string
	Email string
}

type Family struct {
	Surname   string
	Couple    string
	HasCouple bool
	Address   string
	Phone     string
	Email     string
	People    []*Person
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
	if strings.ToLower(email) == strings.ToLower(familyEmail) {
		email = ""
	}

	return email
}

var Spaces = regexp.MustCompile(`\s+`)

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
		for n, person := range family.People {
			if needcomma && family.HasCouple && dir.UseAmpersand && n == 1 {
				// use an ampersand to join spouses
				entry = packBox(entry, "&", 2, dir.Roman)
				needcomma = false
			} else if needcomma {
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
	kinds := []*[]*RegularExpression{
		&dir.PhoneRegexps,
		&dir.AddressRegexps,
		&dir.NameRegexps,
	}
	for _, kind := range kinds {
		old := *kind
		*kind = nil
		for _, elt := range old {
			if strings.TrimSpace(elt.Expression) != "" {
				*kind = append(*kind, elt)
			}
			if elt.Regexp, err = regexp.Compile("(?i:" + elt.Expression + ")"); err != nil {
				elt.Regexp = FallbackRegexp
				elt.Expression = "!!Error!! " + elt.Expression
			}
		}
	}

	return
}
