package actions

import (
	"fmt"

	"github.com/rlf/time_register_cli/internal/db"
)

func GetConfigValue(d *db.DB, key string) (string, error) {
	return d.GetConfig(key)
}

func SetConfigValue(d *db.DB, key, value string) error {
	if err := d.SetConfig(key, value); err != nil {
		return err
	}
	fmt.Printf("Config %s = %s\n", key, value)
	return nil
}

func PrintAllConfig(d *db.DB) error {
	config, err := d.GetAllConfig()
	if err != nil {
		return err
	}

	if len(config) == 0 {
		fmt.Println("No configuration set.")
		return nil
	}

	fmt.Println("Configuration:")
	for k, v := range config {
		fmt.Printf("  %s = %s\n", k, v)
	}
	return nil
}
