package etchosts

import (
	"errors"
	"fmt"
	"os"

	"github.com/abdularis/kportfwd/internal/log"

	"github.com/txn2/txeh"
)

var hostsFile *txeh.Hosts

func Init() {
	hosts, err := txeh.NewHostsDefault()
	if err != nil {
		panic(fmt.Errorf("unable to init hosts file: %w", err))
	}
	hostsFile = hosts
	if err := hostsFile.Save(); err != nil {
		if errors.Is(err, os.ErrPermission) {
			log.Warnf("permission denied on %s, add permission or run as privileged to add local domain for forwarded ports", hostsFile.ReadFilePath)
		}
	}
}

func AddHost(ip, host string) {
	hostsFile.AddHost(ip, host)
	hostsFile.Save()
}

func RemoveHost(host string) {
	hostsFile.RemoveHost(host)
	hostsFile.Save()
}
