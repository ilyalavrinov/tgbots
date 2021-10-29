package cmd

import "log"
import "regexp"
import "strings"

func msgMatches(text string, patterns []string) bool {
	compiledRegExp := make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			log.Printf("Pattern %s cannot be compiled into a regexp. Error: %s", pattern, err)
			continue
		}
		compiledRegExp = append(compiledRegExp, re)
	}

	msgWords := strings.Split(text, " ")
	for _, word := range msgWords {
		for _, re := range compiledRegExp {
			if re.MatchString(strings.ToLower(word)) {
				log.Printf("Word %s matched regexp %s", word, re)
				return true
			}
		}
	}
	log.Printf("None of the words in text: %s; matched patterns %s", text, patterns)
	return false
}
