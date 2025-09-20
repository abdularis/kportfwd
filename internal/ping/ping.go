package ping

import (
	"bytes"
	"fmt"
	"os/exec"
)

func Ping(ipAddr string) error {
	strBuff := bytes.NewBufferString("")

	cmd := exec.Command("ping", "-c", "1", "-t", "1", ipAddr)
	// cmd.Stdout = os.Stdout
	cmd.Stderr = strBuff

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %s", err, strBuff.String())
	}

	return nil
}
