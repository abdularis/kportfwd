package ifconfig

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
)

func AddLoopbackAlias(aliasIpAddr string) error {
	strBuff := bytes.NewBufferString("")
	cmd := exec.Command("ifconfig", "lo0", "alias", aliasIpAddr)
	cmd.Stdout = os.Stdout
	cmd.Stderr = strBuff

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %s", err, strBuff.String())
	}

	return nil
}

func RemoveLoopbackAlias(aliasIpAddr string) error {
	strBuff := bytes.NewBufferString("")

	cmd := exec.Command("ifconfig", "lo0", "-alias", aliasIpAddr)
	cmd.Stdout = os.Stdout
	cmd.Stderr = strBuff

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %s", err, strBuff.String())
	}

	return nil
}
