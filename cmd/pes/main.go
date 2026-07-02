// pes es la CLI del Prompt Engineering Studio. Comparte el 100 % de los casos
// de uso con la futura UI de escritorio: toda la lógica vive en internal/core.
package main

import (
	"fmt"
	"os"

	"github.com/Vergil2599/startup/pes/cmd/pes/cli"
)

func main() {
	if err := cli.Root().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
