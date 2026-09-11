package webhandlers

import (
	"io"
	"os"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestEmailTestButtonHasIndependentForm(t *testing.T) {
	source, err := os.ReadFile("../../../web/templates/email_settings.html")
	if err != nil {
		t.Fatal(err)
	}
	z := html.NewTokenizer(strings.NewReader(string(source)))
	inForm, foundForm, foundButton := false, false, false
	for {
		switch z.Next() {
		case html.ErrorToken:
			if z.Err() != io.EOF {
				t.Fatal(z.Err())
			}
			if !foundForm || !foundButton {
				t.Fatal("missing independent test form or associated button")
			}
			return
		case html.StartTagToken:
			token := z.Token()
			attrs := map[string]string{}
			for _, attr := range token.Attr {
				attrs[attr.Key] = attr.Val
			}
			if token.Data == "form" {
				if inForm {
					t.Fatal("nested form causes browser to submit settings instead of test")
				}
				inForm = true
				if attrs["id"] == "email-test-form" && attrs["action"] == "/settings/email/test" && attrs["method"] == "POST" {
					foundForm = true
				}
			}
			if token.Data == "button" && attrs["form"] == "email-test-form" && attrs["type"] == "submit" {
				foundButton = true
			}
		case html.EndTagToken:
			if z.Token().Data == "form" {
				inForm = false
			}
		}
	}
}
