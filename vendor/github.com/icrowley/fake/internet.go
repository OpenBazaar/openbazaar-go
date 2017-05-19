package fake

import (
	"strconv"
	"strings"
)

// UserName generates user name in one of the following forms
// first name + last name, letter + last names or concatenation of from 1 to 3 lowercased words
func UserName() string {
	gender := randGender()
	switch r.Intn(3) {
	case 0:
		return lookup("en", gender+"_first_names", false) + lookup(lang, gender+"_last_names", false)
	case 1:
		return Character() + lookup(lang, gender+"_last_names", false)
	default:
		return strings.Replace(WordsN(r.Intn(3)+1), " ", "_", -1)
	}
}

// TopLevelDomain generates random top level domain
func TopLevelDomain() string {
	return lookup(lang, "top_level_domains", true)
}

// DomainName generates random domain name
func DomainName() string {
	return Company() + "." + TopLevelDomain()
}

// EmailAddress generates email address
func EmailAddress() string {
	return UserName() + "@" + DomainName()
}

// EmailSubject generates random email subject
func EmailSubject() string {
	return Sentence()
}

// EmailBody generates random email body
func EmailBody() string {
	return Paragraphs()
}

// DomainZone generates random domain zone
func DomainZone() string {
	return lookup(lang, "domain_zones", true)
}

// IPv4 generates IPv4 address
func IPv4() string {
	ip := make([]string, 4)
	for i := 0; i < 4; i++ {
		ip[i] = strconv.Itoa(r.Intn(256))
	}
	return strings.Join(ip, ".")
}
