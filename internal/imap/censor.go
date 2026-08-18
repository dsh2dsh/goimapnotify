package imap

import (
	"bufio"
	"io"
	"log/slog"
	"regexp"
)

var (
	emailRegexp           = regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
	detectPasswordInLOGIN = regexp.MustCompile(`^(?i)(.*LOGIN\s+\S+\s+)"[^"]+"(.*)$`)
	oauthTokenRegexp      = regexp.MustCompile(`(auth=Bearer )([^\x00\x01\r\n\t ]+)`)
)

// censorCredentials pipes input to output while censoring sensitive information
func censorCredentials(in io.Reader, out io.Writer) {
	scanner := bufio.NewScanner(in)
	for scanner.Scan() {
		line := scanner.Text()
		censoredLine := censorEmailAddress(censorOAuthToken(
			censorPasswordInLogin(line)))

		_, err := out.Write([]byte(censoredLine + "\n"))
		if err != nil {
			slog.Error("unable to write censored line", slog.Any("error", err))
		}
	}

	if err := scanner.Err(); err != nil {
		slog.Error("unable to scan censored line", slog.Any("error", err))
	}
}

// censorPasswordInLogin replaces password in LOGIN command
func censorPasswordInLogin(in string) string {
	matches := detectPasswordInLOGIN.FindStringSubmatch(in)
	if len(matches) == 0 {
		return in
	}
	return matches[1] + `"****"` + matches[2]
}

// censorOAuthToken replaces XOAUTH2/OAUTHBEARER bearer tokens with asterisks.
// SASL token payloads appear as "auth=Bearer <token>" in the raw debug stream.
func censorOAuthToken(in string) string {
	return oauthTokenRegexp.ReplaceAllString(in, `${1}****`)
}

// censorEmailAddress replaces email addresses with asterisks
func censorEmailAddress(in string) string {
	matches := emailRegexp.FindAllString(in, -1)
	if len(matches) == 0 {
		return in
	}
	return emailRegexp.ReplaceAllString(in, "*******@*****.***")
}
