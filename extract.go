package main

import (
	"bufio"
	"io"
	"log"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

const (
	black  = "0 0 0 rg"
	dark   = "0.32157 0.32157 0.32157 rg"
	medium = "0.47059 0.47059 0.47059 rg"
	light  = "0.75294 0.75294 0.75294 rg"
)

const (
	titleFont     string = "/F1 16 Tf"
	largeNameFont string = "/F1 10 Tf"
	smallNameFont string = "/F1 7 Tf"
	contactFont   string = "/F2 7 Tf"
	footerFont    string = "/F2 6 Tf"
)

func (dir *Directory) ParseFamilies(src io.Reader) error {
	// the result
	var family *Family
	var person *Person

	// use a state machine to extract families
	instream := false
	font, color := "", ""
	scanner := bufio.NewScanner(src)
	for scanner.Scan() {
		line := scanner.Text()

		// skip everything except page data
		switch line {
		case "stream":
			instream = true
			continue
		case "endstream":
			instream = false
			continue
		}
		if !instream {
			continue
		}

		// capture font changes
		switch line {
		case titleFont, largeNameFont, smallNameFont, contactFont, footerFont:
			font = line
			continue
		case black, dark, medium, light:
			color = line
			continue
		default:
		}

		// ignore everything except text
		if !strings.HasPrefix(line, "(") || !strings.HasSuffix(line, ")Tj") {
			continue
		}
		text := strings.TrimSuffix(strings.TrimPrefix(line, "("), ")Tj")

		// skip letter headers: assume no families have single letter last name
		if len(text) == 1 && font == largeNameFont && color == black {
			continue
		}
		_, _ = color, font

		// skip headers and footers
		if font == titleFont || font == footerFont {
			continue
		}

		// trim leading \t sequences
		for strings.HasPrefix(text, `\t`) {
			text = strings.TrimPrefix(text, `\t`)
			text = Spaces.ReplaceAllString(text, " ")
			text = strings.TrimSpace(text)
		}

		// new family?
		if font == largeNameFont && color == black {
			// create a new record
			family = new(Family)
			dir.Families = append(dir.Families, family)

			family.Surname = text
			continue
		}

		// new adult?
		if font == smallNameFont && color == black {
			// create a new person
			person = new(Person)
			family.People = append(family.People, person)

			if len(family.People) > 1 {
				family.HasCouple = true
				family.Couple += " & " + text
			}
			person.Name = text
			continue
		}

		// new child?
		if font == smallNameFont && color == medium {
			// create a new person
			person = new(Person)
			family.People = append(family.People, person)

			person.Name = text
			continue
		}

		// contact details for family?
		if font == contactFont && len(family.People) == 0 {
			if looksLikeEmail(text) {
				// looks like an email address
				family.Email = text
			} else if looksLikePhone(text) {
				family.Phone = text
			} else {
				family.Address = append(family.Address, text)
			}
			continue
		}

		// contact details for individual?
		if font == contactFont {
			if looksLikeEmail(text) {
				// looks like an email address
				person.Email = text
			} else if looksLikePhone(text) {
				person.Phone = text
			} else {
				log.Printf("Warning: contact for person not recognized: [%s]", text)
			}
			continue
		}

		log.Printf("Warning: unknown text: [%s]", text)
	}
	if err := scanner.Err(); err != nil {
		log.Printf("Error scanning input: %v", err)
		return err
	}
	return nil
}

func looksLikeEmail(s string) bool {
	return strings.Contains(s, "@")
}

func looksLikePhone(s string) bool {
	digits := 0
	for _, r := range s {
		if unicode.IsDigit(r) {
			digits++
		} else if !strings.Contains(`-()\ `, string(r)) {
			return false
		}
	}
	return digits == 7 || digits == 10
}

func (dir *Directory) PrepareFamilies() error {
	// clean up entries details
	for _, f := range dir.Families {
		if dir.FamilyAddress {
			f.Address = prepAddress(dir.AddressRegexps, f.Address)
		} else {
			f.Address = nil
		}

		if dir.FamilyPhone {
			f.Phone = prepPhone(dir.PhoneRegexps, f.Phone, "")
		} else {
			f.Phone = ""
		}

		if dir.FamilyEmail {
			f.Email = prepEmail(f.Email, "")
		} else {
			f.Email = ""
		}

		for _, p := range f.People {
			p.Name = prepName(dir.NameRegexps, p.Name)

			// only show surname if different from family name
			if strings.HasPrefix(strings.ToLower(p.Name), strings.ToLower(f.Surname)+", ") {
				p.Name = p.Name[len(f.Surname)+2:]

				// rearrange as "first last"
				// note: if last name has already been removed, we
				// assume this is "bob, jr" or the like and leave it alone, hence else if
			} else if comma := strings.Index(p.Name, ", "); comma >= 0 {
				p.Name = p.Name[comma+2:] + " " + p.Name[:comma]
			}

			if dir.PersonalPhones {
				p.Phone = prepPhone(dir.PhoneRegexps, p.Phone, f.Phone)
			} else {
				p.Phone = ""
			}

			if dir.PersonalEmails {
				p.Email = prepEmail(p.Email, f.Email)
			} else {
				p.Email = ""
			}
		}
	}

	sort.Sort(familyList(dir.Families))
	return nil
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

func prepAddress(regexps []*RegularExpression, address []string) []string {
	// prepare address
	var out []string
	for _, line := range address {
		for _, re := range regexps {
			line = re.Regexp.ReplaceAllString(strings.TrimSpace(line), re.Replacement)
			line = Spaces.ReplaceAllString(line, " ")
			line = strings.TrimSpace(line)
		}
		if len(line) > 0 {
			out = append(out, line)
		}
	}

	return out
}

var Phone10Digit = regexp.MustCompile(`^\D*(\d{3})\D*(\d{3})\D*(\d{4})\D*$`)
var Phone7Digit = regexp.MustCompile(`^\D*(\d{3})\D*(\d{4})\D*$`)
var Spaces = regexp.MustCompile(`\s+`)

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
