package ifconfig

import (
	"os"
	"os/exec"
)

func AddLoopbackAlias(aliasIpAddr string) error {
	cmd := exec.Command("ifconfig", "lo0", "alias", aliasIpAddr)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func RemoveLoopbackAlias(aliasIpAddr string) error {
	cmd := exec.Command("ifconfig", "lo0", "-alias", aliasIpAddr)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
