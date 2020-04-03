//go:generate mockery -name=^(Plugin)$ -output=./testutil -outpkg=testutil -case=snake

package plugin

import "github.com/bluemedora/bplogagent/entry"

// Plugin is a log monitoring component.
type Plugin interface {
	// ID returns the id of the plugin.
	ID() string
	// Type returns the type of the plugin.
	Type() string

	// Start will start the plugin.
	Start() error
	// Stop will stop the plugin.
	Stop() error

	// CanOutput indicates if the plugin will output entries to other plugins.
	CanOutput() bool
	// Outputs returns the list of connected outputs.
	Outputs() []Plugin
	// SetOutputs will set the connected outputs.
	SetOutputs([]Plugin) error

	// CanProcess indicates if the plugin will process entries from other plugins.
	CanProcess() bool
	// Process will process an entry from a plugin.
	Process(*entry.Entry) error
}
