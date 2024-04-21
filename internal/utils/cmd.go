package utils

import (
	"os"
	"os/exec"
)

func RunGoImports(dir string) error {
	err := os.Chdir(dir)
	if err != nil {
		return err
	}
	cmd := exec.Command("goimports", "-w", ".")

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
