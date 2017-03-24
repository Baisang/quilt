package ssh

//go:generate mockery -name=Client -inpkg

// Client is an SSH client used for `quilt` commands.
type Client interface {
	// Run runs a command over the SSH connection.
	Run(bool, string) error

	// Close closes the SSH connection.
	Close() error

	// Shell creates a login shell.
	Shell() error
}

// Getter is used to retrieve a Client.
type Getter func(string, string) (Client, error)
