// Command sol prints the day's solar schedule (sunrise, solar noon,
// sunset) for a location, with every time shown in the requested time
// zone and labeled with its UTC offset.
package main

import (
	"os"

	"github.com/zaephor-bot/sol-cli/internal/cli"

	// Embed the IANA time zone database so the binary resolves zone names
	// on hosts without /usr/share/zoneinfo. (internal/tz also embeds it,
	// keeping the zone logic self-sufficient for its own tests.)
	_ "time/tzdata"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
