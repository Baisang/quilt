package minion

import (
	"os"

	"github.com/quilt/quilt/db"
	"github.com/quilt/quilt/util"

	log "github.com/Sirupsen/logrus"
)

const authorizedKeysFile = "/home/quilt/.ssh/authorized_keys"

func syncAuthorizedKeys(conn db.Conn) {
	for range conn.TriggerTick(30, db.MinionTable).C {
		if err := runOnce(conn); err != nil {
			log.WithError(err).Error("Failed to sync keys")
		}
	}
}

func runOnce(conn db.Conn) error {
	if _, err := util.AppFs.Stat(authorizedKeysFile); os.IsNotExist(err) {
		util.AppFs.Create(authorizedKeysFile)
	}
	currKeys, err := util.ReadFile(authorizedKeysFile)
	if err != nil {
		return err
	}

	m := conn.MinionSelf()

	if m.AuthorizedKeys == currKeys {
		return nil
	}

	return util.WriteFile(authorizedKeysFile, []byte(m.AuthorizedKeys), 0644)
}
