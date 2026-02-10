package util

import (
	"bufio"
	"io"
	"regexp"

	"github.com/sirupsen/logrus"
)

var (
	emailRegexp           = regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
	detectPasswordInLOGIN = regexp.MustCompile(`^(.*LOGIN\s+\S+\s+)"[^"]+"(.*)$`)
)

// CensorCredentials pipes input to output while censoring sensitive information
func CensorCredentials(in io.Reader, out io.Writer) {
	scanner := bufio.NewScanner(in)
	for scanner.Scan() {
		line := scanner.Text()
		censoredLine := CensorEmailAddress(CensorPasswordInLogin(line))

		_, err := out.Write([]byte(censoredLine + "\n"))
		if err != nil {
			logrus.WithError(err).Error("unable to write censored line")
		}
	}
}

// CensorPasswordInLogin replaces password in LOGIN command
func CensorPasswordInLogin(in string) string {
	matches := detectPasswordInLOGIN.FindStringSubmatch(in)

	if len(matches) == 0 {
		return in
	}

	return matches[1] + `"****"` + matches[2]
}

// CensorEmailAddress replaces email addresses with asterisks
func CensorEmailAddress(in string) string {
	matches := emailRegexp.FindAllString(in, -1)

	if len(matches) == 0 {
		return in
	}

	return emailRegexp.ReplaceAllString(in, "*******@*****.***")
}
