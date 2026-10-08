package service

import (
	"io"
	"mime"
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

func TestMIMEIncludesDeliverabilityHeadersAndEncodesSubject(t *testing.T) {
	raw := buildMIMEMessage("alertas@example.com", []string{"to@example.com"}, "Probakgo: copia fallida en nodo-ñ", "<p>x</p>")
	message, err := mail.ReadMessage(strings.NewReader(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := message.Header.Date(); err != nil {
		t.Fatalf("Date header: %v", err)
	}
	if id := message.Header.Get("Message-ID"); !strings.HasPrefix(id, "<") || !strings.HasSuffix(id, "@example.com>") {
		t.Fatalf("Message-ID header: %q", id)
	}
	subject, err := new(mime.WordDecoder).DecodeHeader(message.Header.Get("Subject"))
	if err != nil || subject != "Probakgo: copia fallida en nodo-ñ" {
		t.Fatalf("subject: %q %v (raw %q)", subject, err, message.Header.Get("Subject"))
	}
}
