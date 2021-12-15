package main

type Configuration struct {
	Host           string   `envconfig:"HOST" required:"true" default:":8080"`
	OriginPatterns []string `envconfig:"ORIGIN_PATTERNS" desc:"the list of host patterns for authorized origins."`
}

var config Configuration
