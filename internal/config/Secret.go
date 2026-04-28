package config

import (
	"encoding"
	"encoding/json"

	"github.com/ilyakaznacheev/cleanenv"
)

const maskValue = "********"

type Secret string

var (
	_ encoding.TextMarshaler   = Secret("")
	_ encoding.TextUnmarshaler = (*Secret)(nil)
	_ json.Marshaler           = Secret("")
	_ cleanenv.Setter          = (*Secret)(nil)
)

func (s Secret) String() string {
	if s == "" {
		return ""
	}
	return maskValue
}

// RawContents exposes unmasked contents of the secret. Same as string(s)
func (s Secret) RawContents() string {
	return string(s)
}

func (s Secret) IsZero() bool {
	return s == ""
}

// MarshalText implements [encoding.TextMarshaler].
func (s Secret) MarshalText() (text []byte, err error) {
	if s == "" {
		return []byte(""), nil
	}
	return []byte(maskValue), nil
}

// UnmarshalText implements [encoding.TextUnmarshaler].
func (s *Secret) UnmarshalText(text []byte) error {
	*s = Secret(text)
	return nil
}

// MarshalJSON implements [json.Marshaler]
func (s Secret) MarshalJSON() ([]byte, error) {
	if s == "" {
		return []byte(`""`), nil
	}
	return json.Marshal(maskValue)
}

// SetValue implements [cleanenv.Setter].
func (s *Secret) SetValue(v string) error {
	*s = Secret(v)
	return nil
}
