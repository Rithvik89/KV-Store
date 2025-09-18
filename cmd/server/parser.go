package main

import "strings"

const (
	CMD_GET = "GET"
	CMD_PUT = "PUT"
)

func parseCmd(cmd string) []string {
	return strings.Split(cmd, " ")
}

func validateCmd(args []string) bool {
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

func parseAndValidateCmd(cmd string) ([]string, bool) {
	args := parseCmd(cmd)
	isValid := validateCmd(args)

	return args, isValid
}
