package main

import (
	"flag"
	"os"
	"os/signal"

	"net/http"
	_ "net/http/pprof"

	bpla "github.com/bluemedora/bplogagent"
	"github.com/bluemedora/bplogagent/config"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func main() {
	logCfg := zap.NewProductionConfig()
	logCfg.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	// logCfg := zap.NewDevelopmentConfig()
	// logCfg.EncoderConfig.TimeKey = ""
	// logCfg.EncoderConfig.CallerKey = ""
	// logCfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	baseLogger, _ := logCfg.Build()
	logger := baseLogger.Sugar()
	defer func() {
		_ = logger.Sync()
	}()

	var configFile string
	flag.StringVar(&configFile, "config", "/etc/bplogagent/config.yml", "Path to the config file")
	flag.Parse()

	var cfg config.Config
	v := viper.New()
	v.SetConfigFile(configFile)
	err := v.ReadInConfig()
	if err != nil {
		logger.Errorw("Failed to read the config", zap.Error(err))
		return
	}
	err = v.Unmarshal(&cfg, func(cfg *mapstructure.DecoderConfig) {
		cfg.DecodeHook = config.DecodeHookFunc
	})
	if err != nil {
		logger.Errorw("Failed to unmarshal the config", "error", err)
		return
	}

	// cfgYaml, err := yaml.Marshal(cfg)
	// if err != nil {
	// 	logger.Errorw("Failed to marshal yaml", "error", err)
	// }
	// logger.Infof("Unmarshalled the config:\n%s\n", string(cfgYaml))

	agent := bpla.NewLogAgent(&cfg, logger)

	err = agent.Start()
	if err != nil {
		logger.Errorw("Failed to start log collector", zap.Error(err))
		return
	}

	// Start the profiler http server
	go func() {
		logger.Info(http.ListenAndServe("localhost:6060", nil))
	}()

	// Wait for interrupt to exit
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	<-interrupt
	agent.Stop()
}
