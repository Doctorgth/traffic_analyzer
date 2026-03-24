package main

import (
	"encoding/json"
	"os"
)

type Config struct {
	ProxyPort  string `json:"proxy_port"` // 1050
	GamePort   string `json:"game_port"`  // 1239
	User       string `json:"user"`       // main
	Pass       string `json:"pass"`       // 1357
	ShowTime   bool   `json:"show_time"`
	ShowLen    bool   `json:"show_len"`
	ShowOpcode bool   `json:"show_opcode"`
	ShowData   bool   `json:"show_data"`
}

func LoadConfig() Config {
	conf := Config{
		ProxyPort: "1050", GamePort: "1239",
		User: "main", Pass: "1357",
		ShowTime: true, ShowLen: true, ShowOpcode: true, ShowData: true,
	}
	file, err := os.ReadFile("config.json")
	if err == nil {
		json.Unmarshal(file, &conf)
	}
	return conf
}

func (c *Config) Save() {
	data, _ := json.MarshalIndent(c, "", "  ")
	os.WriteFile("config.json", data, 0644)
}
