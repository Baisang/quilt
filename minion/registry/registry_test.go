package registry

import (
	"testing"

	dkc "github.com/fsouza/go-dockerclient"
	"github.com/stretchr/testify/assert"

	"github.com/quilt/quilt/db"
	"github.com/quilt/quilt/minion/docker"
)

func TestRunMaster(t *testing.T) {
	md, dk := docker.NewMock()
	conn := db.New()

	// Test only run on masters.
	conn.Txn(db.AllTables...).Run(func(view db.Database) error {
		m := view.InsertMinion()
		m.Self = true
		m.Role = db.Worker
		m.PrivateIP = "8.8.8.8"
		view.Commit(m)

		im := view.InsertImage()
		im.Name = "image"
		view.Commit(im)
		return nil
	})
	runOnce(conn, dk)
	assert.Equal(t, md.Built, map[docker.BuildImageOptions]struct{}{},
		"should not attempt to build on worker")

	// Test building an image.
	conn.Txn(db.AllTables...).Run(func(view db.Database) error {
		m := view.SelectFromMinion(nil)[0]
		m.Role = db.Master
		view.Commit(m)
		return nil
	})
	runOnce(conn, dk)

	images := getImages(conn)
	assert.Len(t, images, 1)
	builtID := images[0].DockerID
	assert.NotEmpty(t, builtID, "should save ID of built image")

	// Test ignoring already-built image.
	md.ResetBuilt()
	runOnce(conn, dk)

	images = getImages(conn)
	assert.Len(t, images, 1)
	assert.Equal(t, builtID, images[0].DockerID, "should not change image ID")
	assert.Equal(t, md.Built, map[docker.BuildImageOptions]struct{}{},
		"should not attempt to rebuild")
}

func TestUpdateRegistry(t *testing.T) {
	md, dk := docker.NewMock()

	_, err := updateRegistry(dk, db.Image{
		Name:       "mean:tag",
		Dockerfile: "dockerfile",
	})
	assert.NoError(t, err)

	assert.Equal(t, map[docker.BuildImageOptions]struct{}{
		{
			Name:       "localhost:5000/mean:tag",
			Dockerfile: "dockerfile",
		}: {},
	}, md.Built)

	assert.Equal(t, map[dkc.PushImageOptions]struct{}{
		{
			Registry: "localhost:5000",
			Name:     "localhost:5000/mean",
			Tag:      "tag",
		}: {},
	}, md.Pushed)
}

func getImages(conn db.Conn) (images []db.Image) {
	conn.Txn(db.AllTables...).Run(func(view db.Database) error {
		images = view.SelectFromImage(nil)
		return nil
	})
	return images
}
