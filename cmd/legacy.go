package cmd

import "fmt"

const legacyCommandWarning = "WARN: this command is legacy and will be deprecated soon."

func printLegacyCommandWarning() {
	fmt.Println(legacyCommandWarning)
}
