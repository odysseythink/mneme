package dashboard

import "os/exec"

func execCommand(bin string, args ...string) error {
	c := exec.Command(bin, args...)
	return c.Start()
}
