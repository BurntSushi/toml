package main

import (
	"fmt"
	"time"

	"github.com/BurntSushi/toml"
)

type tomlConfig struct {
	Title string `toml:"title"`
	Owner ownerInfo `toml:"owner"`
	DB database `toml:"database"`
	Servers map[string]server `toml:"servers"`
	Clients clients `toml:"clients"`
}

type ownerInfo struct {
	Name string `toml:"name"`
	Org string `toml:"organization"`
	Bio string `toml:"bio"`
	DOB time.Time `toml:"dob"`
}

type database struct {
	Server string `toml:"server"`
	Ports []int `toml:"ports"`
	ConnMax int `toml:"connection_max"`
	Enabled bool `toml:"enabled"`
}

type server struct {
	IP string `toml:"ip"`
	DC string `toml:"dc"`
}

type clients struct {
	Data [][]interface{} `toml:"data"`
	Hosts []string `toml:"hosts"`
}


func main() {
	var config tomlConfig
	if err := toml.DecodeFile("example.toml", &config); err != nil {
		fmt.Println(err)
		return
	}

	fmt.Printf("Title: %s\n", config.Title)
	fmt.Printf("Owner: %s (%s, %s), Born: %s\n",
		config.Owner.Name, config.Owner.Org, config.Owner.Bio, config.Owner.DOB)
	fmt.Printf("Database: %s %v (Max conn. %d), Enabled? %v\n",
		config.DB.Server, config.DB.Ports, config.DB.ConnMax, config.DB.Enabled)
	for serverName, server := range config.Servers {
		fmt.Printf("Server: %s (%s, %s)\n", serverName, server.IP, server.DC)
	}
	fmt.Printf("Client data: %v\n", config.Clients.Data)
	fmt.Printf("Client hosts: %v\n", config.Clients.Hosts)
}
