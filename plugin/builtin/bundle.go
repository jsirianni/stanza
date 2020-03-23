package builtin

import (
	"fmt"

	"github.com/bluemedora/bplogagent/bundle"
	"github.com/bluemedora/bplogagent/entry"
	pg "github.com/bluemedora/bplogagent/plugin"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func init() {
	pg.RegisterConfig("bundle", &BundleConfig{})
}

type BundleConfig struct {
	pg.DefaultPluginConfig    `mapstructure:",squash" yaml:",inline"`
	pg.DefaultOutputterConfig `mapstructure:",squash" yaml:",inline"`

	BundleType string `mapstructure:"bundle_type" yaml:"bundle_type"`
	Params     map[string]interface{}
}

func (c BundleConfig) Build(buildContext pg.BuildContext) (pg.Plugin, error) {
	configs, err := c.renderPluginConfigs(buildContext.Bundles)
	if err != nil {
		return nil, fmt.Errorf("render bundle configs: %s", err)
	}

	// TODO comment this because it's complicated
	buildContext.IsBundle = true
	defaultBundle, err := c.buildDefaultBundle(configs, buildContext)
	if err != nil {
		return nil, fmt.Errorf("build default bundle: %s", err)
	}

	plugins := defaultBundle.plugins.SortedPlugins()
	bundleInput, bundleOutput, err := findBundleInputOutput(plugins)
	if err != nil {
		return nil, err
	}

	var plugin pg.Plugin
	switch {
	case bundleInput != nil && bundleOutput != nil:
		defaultOutputter, err := c.DefaultOutputterConfig.Build(buildContext.Plugins)
		if err != nil {
			return nil, fmt.Errorf("build default outputter: %s", err)
		}
		bundle := &BothputterBundle{
			DefaultBundle:    defaultBundle,
			DefaultOutputter: defaultOutputter,
			bundleInput:      bundleInput,
		}
		bundleOutput.bundle = bundle
		plugin = bundle
	case bundleInput != nil && bundleOutput == nil:
		plugin = &InputterBundle{
			DefaultBundle: defaultBundle,
			bundleInput:   bundleInput,
		}
	case bundleInput == nil && bundleOutput != nil:
		defaultOutputter, err := c.DefaultOutputterConfig.Build(buildContext.Plugins)
		if err != nil {
			return nil, fmt.Errorf("build default outputter: %s", err)
		}
		bundle := &OutputterBundle{
			DefaultBundle:    defaultBundle,
			DefaultOutputter: defaultOutputter,
		}
		bundleOutput.bundle = bundle
		plugin = bundle
	case bundleInput == nil && bundleOutput == nil:
		plugin = &NeitherputterBundle{
			DefaultBundle: defaultBundle,
		}
	}

	return plugin, nil
}

func (c BundleConfig) renderPluginConfigs(bundles []*bundle.BundleDefinition) ([]pg.PluginConfig, error) {
	var bundleDefinition *bundle.BundleDefinition
	for _, bundle := range bundles {
		if c.BundleType == bundle.BundleType {
			bundleDefinition = bundle
			break // TODO warn on duplicate
		}
	}
	if bundleDefinition == nil {
		return nil, fmt.Errorf("bundle definition with type %s not found in bundle path", c.BundleType)
	}

	// Render the bundle config
	renderedConfig, err := bundleDefinition.Render(c.Params)
	if err != nil {
		return nil, fmt.Errorf("render bundle config: %s", err)
	}

	// Parse the rendered config
	// TODO reuse this code
	v := viper.New()
	v.SetConfigType("yaml")
	err = v.ReadConfig(renderedConfig)
	if err != nil {
		return nil, fmt.Errorf("read config into viper: %s", err)
	}
	var pluginUnmarshaller struct {
		Plugins []pg.PluginConfig
	}
	err = v.UnmarshalExact(&pluginUnmarshaller, pg.UnmarshalHook)
	if err != nil {
		return nil, fmt.Errorf("unmarshal from viper: %s", err)
	}

	return pluginUnmarshaller.Plugins, nil
}

func (c BundleConfig) buildDefaultBundle(configs []pg.PluginConfig, buildContext pg.BuildContext) (DefaultBundle, error) {
	defaultPlugin, err := c.DefaultPluginConfig.Build(buildContext.Logger)
	if err != nil {
		return DefaultBundle{}, fmt.Errorf("build default plugin: %s", err)
	}

	// Clear plugins before build
	configGraph, err := pg.NewPluginConfigGraph(configs)
	if err != nil {
		return DefaultBundle{}, err
	}

	buildContext.Plugins = make(map[pg.PluginID]pg.Plugin)
	pluginGraph, err := configGraph.Build(buildContext)
	if err != nil {
		return DefaultBundle{}, err
	}

	defaultBundle := DefaultBundle{
		bundleType:    c.BundleType,
		plugins:       pluginGraph,
		DefaultPlugin: defaultPlugin,
		SugaredLogger: buildContext.Logger,
	}

	return defaultBundle, nil
}

func findBundleInputOutput(plugins []pg.Plugin) (bundleInput *BundleInput, bundleOutput *BundleOutput, err error) {
	for _, plugin := range plugins {
		switch c := plugin.(type) {
		case *BundleInput:
			if bundleInput != nil {
				return nil, nil, fmt.Errorf("only one plugin of type bundle_input can exist in a bundle")
			}
			bundleInput = c
		case *BundleOutput:
			if bundleOutput != nil {
				return nil, nil, fmt.Errorf("only one plugin of type bundle_output can exist in a bundle")
			}
			bundleOutput = c
		}
	}
	return
}

type DefaultBundle struct {
	bundleType string
	plugins    *pg.PluginGraph

	pg.DefaultPlugin
	*zap.SugaredLogger
}

func (b *DefaultBundle) Start() error {
	return b.plugins.Start()
}

func (b *DefaultBundle) Stop() error {
	// TODO return error if plugins fail to stop
	b.plugins.Stop()
	return nil
}

type InputterBundle struct {
	DefaultBundle

	bundleInput BundleInputter
}

type OutputterBundle struct {
	DefaultBundle
	pg.DefaultOutputter
}

type NeitherputterBundle struct {
	DefaultBundle
}

type BothputterBundle struct {
	DefaultBundle
	pg.DefaultOutputter

	bundleInput BundleInputter
}

func (b *BothputterBundle) Input(entry *entry.Entry) error {
	return b.bundleInput.InputFromBundle(entry)
}

type BundleInputter interface {
	InputFromBundle(entry *entry.Entry) error
}

type BundleOutputter interface {
	Output(entry *entry.Entry) error
}