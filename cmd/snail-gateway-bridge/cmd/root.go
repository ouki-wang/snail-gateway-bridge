package cmd

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"reflect"
	"snail-gateway-bridge/internal/config"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string // config file
var version string

var rootCmd = &cobra.Command{
	Use:   "snail-gateway-bridge",
	Short: "abstracts the packet_forwarder protocol into Protobuf or JSON over MQTT",
	Long: `Snail Gateway Bridge abstracts the packet_forwarder protocol into Protobuf or JSON over MQTT
	> documentation & support: https://www.chirpstack.io/gateway-bridge/
	> source & copyright information: https://github.com/brocaar/chirpstack-gateway-bridge`,
	RunE: run,
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "path to configuration file(optional)")
	rootCmd.PersistentFlags().Int("log-level", 4, "debug=5, info=4, error=2, fatal=1, panic=0")

	viper.BindPFlag("general.log_level", rootCmd.PersistentFlags().Lookup("log-level"))
	log.Info(cfgFile)
	rootCmd.AddCommand(versionCmd)
}

func Execute(v string) {
	rootCmd.Execute()
}

func initConfig() {
	if cfgFile != "" {
		b, err := ioutil.ReadFile(cfgFile)
		if err != nil {
			log.WithError(err).WithField("config", cfgFile).Fatal("error loading config file1")
		}

		viper.SetConfigType("toml")
		if err := viper.ReadConfig(bytes.NewBuffer(b)); err != nil {
			log.WithError(err).WithField("config", cfgFile).Fatal("error loading config file2")
		}

	} else {
		viper.SetConfigName("snail-gateway-bridge")
		viper.AddConfigPath(".")
	}

	if err := viper.ReadInConfig(); err != nil {
		switch err.(type) {
		case viper.ConfigFileNotFoundError:
			log.WithError(err).Fatal("ConfigFileNotFoundError")
		default:
			log.WithError(err).Fatal("read configuration file error")
		}
	}
	/*for _, pair := range os.Environ() {
		d := strings.SplitN(pair, "=", 2)
		underscoreName := strings.ReplaceAll(d[0], ".", "__")
		if _, exists := os.LookupEnv(underscoreName); !exists {
			fmt.Println(underscoreName)
		}

	}*/
	for _, k := range viper.GetViper().AllKeys() {
		fmt.Println(k, viper.GetString(k))
	}
	viperBindEnvs(config.C)
	fmt.Println("----------------------")
	for _, k := range viper.GetViper().AllKeys() {
		fmt.Println(k, viper.GetString(k))
	}
	version = "v0.0.1"
}

func viperBindEnvs(iface interface{}, parts ...string) {
	ifv := reflect.ValueOf(iface)
	ift := reflect.TypeOf(iface)

	for i := 0; i < ift.NumField(); i++ {
		v := ifv.Field(i)
		t := ift.Field(i)

		tv, ok := t.Tag.Lookup("mapstructure")
		if !ok {
			tv = strings.ToLower(t.Name)
		}
		if tv == "-" {
			continue
		}

		switch v.Kind() {
		case reflect.Struct:
			viperBindEnvs(v.Interface(), append(parts, tv)...)
		default:
			// Bash doesn't allow env variable names with a dot so
			// bind the double underscore version.
			keyDot := strings.Join(append(parts, tv), ".")
			keyUnderscore := strings.Join(append(parts, tv), "__")
			viper.BindEnv(keyDot, strings.ToUpper(keyUnderscore))
		}
	}
}
