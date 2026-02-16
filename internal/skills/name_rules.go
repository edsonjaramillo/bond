package skills

import (
	"regexp"
	"unicode/utf8"
)

const SkillNameMaxRunes = 64

var skillNamePattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// SkillNameCheck captures canonical skill-name rule evaluations.
type SkillNameCheck struct {
	RuneCount     int
	Empty         bool
	TooLong       bool
	InvalidFormat bool
}

// CheckSkillName evaluates skill-name rules shared by create and validate flows.
func CheckSkillName(name string) SkillNameCheck {
	runeCount := utf8.RuneCountInString(name)
	check := SkillNameCheck{RuneCount: runeCount}

	if runeCount == 0 {
		check.Empty = true
		return check
	}
	if runeCount > SkillNameMaxRunes {
		check.TooLong = true
	}
	if !skillNamePattern.MatchString(name) {
		check.InvalidFormat = true
	}

	return check
}
