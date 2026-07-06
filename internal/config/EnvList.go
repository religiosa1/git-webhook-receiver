package config

import (
	"encoding"
	"encoding/json"
)

// EnvList is a list of "KEY=VALUE" environment entries. Its contents are
// considered sensitive (values may hold tokens/credentials), so -- like
// [Secret] -- it masks itself in every rendered form: JSON (API/UI, DB-stored
// pipeline config), text and the slog config dump. Only the receiver's own code
// reads the raw entries (it's assignable to []string), everything user-facing
// sees the mask.
type EnvList []string

var (
	_ encoding.TextMarshaler = EnvList(nil)
	_ json.Marshaler         = EnvList(nil)
)

// String implements [fmt.Stringer], covering the slog text handler which
// fmt-prints the surrounding config struct field by field.
func (e EnvList) String() string {
	if len(e) == 0 {
		return ""
	}
	return maskValue
}

// MarshalJSON implements [json.Marshaler]. A non-empty list collapses to a
// single masked entry so neither the values nor the count leak.
func (e EnvList) MarshalJSON() ([]byte, error) {
	if len(e) == 0 {
		return []byte("[]"), nil
	}
	return json.Marshal([]string{maskValue})
}

// MarshalText implements [encoding.TextMarshaler] for the text slog handler and
// any other text-based serialization.
func (e EnvList) MarshalText() ([]byte, error) {
	if len(e) == 0 {
		return []byte(""), nil
	}
	return []byte(maskValue), nil
}
