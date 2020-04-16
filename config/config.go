package config

import (
	"bytes"
	"io/ioutil"
	"log"
	"strings"

	"github.com/spf13/viper"
)

var Config AppConfig

type AppConfig struct {
	SQL struct {
		Host                    string                       `mapstructure:"host"`
		Port                    int                          `mapstructure:"port"`
		User                    string                       `mapstructure:"user"`
		Password                string                       `mapstructure:"password"`
		SchemaPath              string                       `mapstructure:"schema_path"`
		TableMeta               map[string]map[string]string `mapstructure:"table_meta"`
		TrasactionIDPlaceholder string                       `mapstructure:"trasaction_id_placeholder"`
	} `mapstructure:"sql"`

	Models struct {
		Members               map[string]int `mapstructure:"members"`
		Posts                 map[string]int `mapstructure:"posts"`
		PostType              map[string]int `mapstructure:"post_type"`
		PostPublishStatus     map[string]int `mapstructure:"post_publish_status"`
		ProjectsActive        map[string]int `mapstructure:"projects_active"`
		ProjectsPublishStatus map[string]int `mapstructure:"projects_publish_status"`
		FollowingType         map[string]int `mapstructure:"following_type"`
		Emotions              map[string]int `mapstructure:"emotions"`
	} `mapstructure:"models"`

	DomainName string `mapstructure:"domain_name"`
}

func LoadConfig(configPath string) (conf AppConfig, err error) {

	viper.SetConfigType("json")
	viper.AutomaticEnv()        // read in environment variables that match
	viper.SetEnvPrefix("READR") // READR_
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if configPath != "" {
		content, err := ioutil.ReadFile(configPath)

		if err != nil {
			return Config, err
		}

		if err := viper.ReadConfig(bytes.NewBuffer(content)); err != nil {
			return Config, err
		}

		if err := viper.Unmarshal(&Config); err != nil {
			log.Fatalf("Error unmarshal config file, %s", err)
			return Config, err
		}

	} else {
		// Default path
		viper.AddConfigPath("./config/")
		viper.SetConfigName("main")

		if err := viper.ReadInConfig(); err != nil {
			log.Fatalf("Error reading config file, %s", err)
			return Config, err
		}
		log.Println("Using config file:", viper.ConfigFileUsed())

		if err := viper.Unmarshal(&Config); err != nil {
			log.Fatalf("Error unmarshal config file, %s", err)
			return Config, err
		}
	}

	return Config, nil
}
