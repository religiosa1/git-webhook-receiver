//go:build !integration

package views

import "github.com/a-h/templ"

// TestID sets data-testid  html attrivute to the provided value ONLY in
// integration tests build. In other builds it does nothing.
func TestID(_ string) templ.Attributes {
	return templ.Attributes{}
}
