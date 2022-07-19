package agent

import (
	jsoniter "github.com/json-iterator/go"
	"github.com/miekg/pkcs11"
	"log"
	"os"
)

type Card struct {
	Vendor  string
	Product string
	Serial  string
	Info    pkcs11.TokenInfo
}

type Config struct {
	TokenDriver     map[string]map[string]string // tokenDriver of type [OS] -> [vendor] -> pathToDriver
	Path            string                       // path to config
	ReloadTime      string                       // reloadTime from servers
	PossibleVendors []string                     // possibleVendors of tokens
}

func (config *Config) Update() {
	marshaled, err := jsoniter.Marshal(config)

	if err != nil {
		return
	}

	configFile, _ := os.Open(config.Path)
	if config.Path != "" {
		if configFile != nil {
			log.Printf("%s: %s", "Config", "Was found at "+config.Path)
			configBytes, _ := os.ReadFile(config.Path)
			err := jsoniter.Unmarshal(configBytes, config)
			if err != nil {
				return
			}
		} else {
			configFile, err := os.Create(config.Path)
			_, err = configFile.Write(marshaled)
			if err != nil {
				return
			}
		}
		return
	} else {
		// Search config in project directory
		configFile, err = os.Open("config.json")
		if configFile != nil {
			log.Printf("%s: %s", "Config", "Was found at "+configFile.Name())
			configBytes, _ := os.ReadFile(configFile.Name())
			err := jsoniter.Unmarshal(configBytes, config)
			if err != nil {
				return
			}
		}
		return
	}
}
