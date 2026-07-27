package util

import (
	"bufio"
	"io"
	"log/slog"
	"regexp"
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
			slog.Error("unable to write censored line", slog.Any("error", err))
		}
	}

	if err := scanner.Err(); err != nil {
		slog.Error("unable to scan censored line", slog.Any("error", err))
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
