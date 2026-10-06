//go:build !windows

package main

import "log"

func showError(title, msg string) { log.Printf("%s: %s", title, msg) }
