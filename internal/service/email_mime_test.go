package service

import (
	"io"
	"mime/quotedprintable"
	"net/mail"
	"strings"
	"testing"
)

func TestMIMEPreservesLongHTML(t *testing.T) {
	html := `<html><body>` + strings.Repeat(`<table width="100%"><tr><td style="font-size:19px">Incidencia resuelta: ñ</td></tr></table>`, 100) + `</body></html>`
	raw := buildMIMEMessage("from@example.com", []string{"to@example.com"}, "Incidencias", html)
	for _, line := range strings.Split(string(raw), "\r\n") {
		if len(line) > 998 {
			t.Fatalf("SMTP line too long: %d", len(line))
		}
	}
	message, err := mail.ReadMessage(strings.NewReader(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	if message.Header.Get("Content-Transfer-Encoding") != "quoted-printable" {
		t.Fatal("missing MIME encoding")
	}
	decoded, err := io.ReadAll(quotedprintable.NewReader(message.Body))
	if err != nil || string(decoded) != html {
		t.Fatal("transport changed HTML", err)
	}
}
