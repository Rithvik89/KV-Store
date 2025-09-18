package parser

import "strings"

const (
	CMD_GET = "GET"
	CMD_PUT = "PUT"
)

// ParseCmd splits a command string into arguments
func ParseCmd(cmd string) []string {
	return strings.Split(cmd, " ")
}

// ValidateCmd checks if the command arguments are valid
func ValidateCmd(args []string) bool {
	if len(args) > 1 {
		if args[0] == CMD_GET && len(args) == 2 {
			return true
		}
		if args[0] == CMD_PUT && len(args) == 3 {
			return true
		}
	}
	return false
}

// ParseAndValidateCmd parses and validates a command in one step
func ParseAndValidateCmd(cmd string) ([]string, bool) {
	args := ParseCmd(cmd)
	isValid := ValidateCmd(args)

	return args, isValid
}