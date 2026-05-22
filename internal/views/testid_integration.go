//go:build integration

package views

import "github.com/a-h/templ"

func TestID(name string) templ.Attributes {
	return templ.Attributes{"data-testid": name}
}
