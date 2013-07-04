package main

import "log"
import "github.com/BurntSushi/toml"

var configFile = "config.toml"

type Config struct {
	Server ServerConfig `toml:"server"`
	Board  BoardConfig  `toml:"board"`
	DB     DBConfig     `toml:"database"`
}

type ServerConfig struct {
	Bind string
	Path string
}

type BoardConfig struct {
	Name string
	Desc string
}

type DBConfig struct {
	Addr string
	Name string
}

func readConfig() Config {
	var cfg Config
	_, err := toml.DecodeFile(configFile, &cfg)
	if err != nil {
		log.Fatalf("Couldn't read config file: %s", err.Error())
	}
	return cfg
}
