// Package i18n holds the current interface language (English or French).
package i18n

import "sync/atomic"

// Lang is an interface language.
type Lang int32

const (
	EN Lang = iota
	FR
)

var cur atomic.Int32 // EN by default

// Current returns the active language.
func Current() Lang { return Lang(cur.Load()) }

// Set changes the active language.
func Set(l Lang) { cur.Store(int32(l)) }

// Toggle switches between English and French and returns the new language.
func Toggle() Lang {
	l := EN
	if Current() == EN {
		l = FR
	}
	Set(l)
	return l
}

// S returns en or fr depending on the active language.
func S(en, fr string) string {
	if Current() == FR {
		return fr
	}
	return en
}

// Name is the language's own name ("English", "Français").
func (l Lang) Name() string {
	if l == FR {
		return "Français"
	}
	return "English"
}

// Code is the short language code ("EN", "FR").
func (l Lang) Code() string {
	if l == FR {
		return "FR"
	}
	return "EN"
}
