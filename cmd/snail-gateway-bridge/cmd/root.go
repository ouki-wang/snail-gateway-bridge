package cmd

import (
	"bytes"
	"io/ioutil"
	"reflect"
	"snail-gateway-bridge/internal/config"
	"strings"
	"time"

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

	// default values
	viper.SetDefault("general.log_level", 4)
	viper.SetDefault("backend.type", "semtech_udp")
	viper.SetDefault("backend.semtech_udp.udp_bind", "0.0.0.0:1700")

	viper.SetDefault("backend.concentratord.crc_check", true)
	viper.SetDefault("backend.concentratord.event_url", "ipc:///tmp/concentratord_event")
	viper.SetDefault("backend.concentratord.command_url", "ipc:///tmp/concentratord_command")

	viper.SetDefault("backend.basic_station.bind", ":3001")
	viper.SetDefault("backend.basic_station.stats_interval", time.Second*30)
	viper.SetDefault("backend.basic_station.ping_interval", time.Minute)
	viper.SetDefault("backend.basic_station.timesync_interval", time.Hour)
	viper.SetDefault("backend.basic_station.read_timeout", time.Minute+(5*time.Second))
	viper.SetDefault("backend.basic_station.write_timeout", time.Second)
	viper.SetDefault("backend.basic_station.region", "EU868")
	viper.SetDefault("backend.basic_station.frequency_min", 863000000)
	viper.SetDefault("backend.basic_station.frequency_max", 870000000)

	viper.SetDefault("integration.marshaler", "protobuf")
	viper.SetDefault("integration.mqtt.auth.type", "generic")

	viper.SetDefault("integration.mqtt.event_topic_template", "gateway/{{ .GatewayID }}/event/{{ .EventType }}")
	viper.SetDefault("integration.mqtt.state_topic_template", "gateway/{{ .GatewayID }}/state/{{ .StateType }}")
	viper.SetDefault("integration.mqtt.command_topic_template", "gateway/{{ .GatewayID }}/command/#")
	viper.SetDefault("integration.mqtt.state_retained", true)
	viper.SetDefault("integration.mqtt.keep_alive", 30*time.Second)
	viper.SetDefault("integration.mqtt.max_reconnect_interval", time.Minute)
	viper.SetDefault("integration.mqtt.max_token_wait", time.Second)

	viper.SetDefault("integration.mqtt.auth.generic.servers", []string{"tcp://127.0.0.1:1883"})
	viper.SetDefault("integration.mqtt.auth.generic.clean_session", true)

	viper.SetDefault("integration.mqtt.auth.gcp_cloud_iot_core.server", "ssl://mqtt.googleapis.com:8883")
	viper.SetDefault("integration.mqtt.auth.gcp_cloud_iot_core.jwt_expiration", time.Hour*24)

	viper.SetDefault("integration.mqtt.auth.azure_iot_hub.sas_token_expiration", 24*time.Hour)

	viper.SetDefault("meta_data.dynamic.split_delimiter", "=")
	viper.SetDefault("meta_data.dynamic.execution_interval", time.Minute)
	viper.SetDefault("meta_data.dynamic.max_execution_duration", time.Second)

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
	//for _, k := range viper.GetViper().AllKeys() {
	//	fmt.Println(k, viper.GetString(k))
	//}
	viperBindEnvs(config.C)

	if err := viper.Unmarshal(&config.C); err != nil {
		log.WithError(err).Fatal("unmarshal config error")
	}

	//fmt.Println("----------------------", config.C.Integration.MQTT.Auth.Generic)
	//for _, k := range viper.GetViper().AllKeys() {
	//	fmt.Println(k, viper.GetString(k))
	//}
	if config.C.Integration.MQTT.Auth.Generic.Server != "" {
		config.C.Integration.MQTT.Auth.Generic.Servers = []string{config.C.Integration.MQTT.Auth.Generic.Server}
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
