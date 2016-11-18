# Go jsconfig

Kailash Nadh, January 2015

MIT License

## What?
jsconfig is a tiny JSON configuration file parser for Go with support for comments. Really, JSON spec doesn't include comments, but a configuration file without helpful comments is a pain to deal with.

Moreover, JSON for configuration files is powerful when combined with structs, enabling effortless loading of complex, nested data structures with Go's native JSON Unmarshaling.

## Installation (go 1.1+)
`go get github.com/knadh/jsonconfig`

## Usage
### Sample file: config.json
Notice the comments
<pre>
{
	// url to the site
	"url": "http://google.com",

	"methods": ["GET", "POST"], // supported methods

	"always_load": true,

	// nested structure with different types
	"module": {
		"name": "Welcome",
		"route": "/",
		"port": 8080
	}
}
</pre>

### Loading the configuration
```go
package main

import (
	"github.com/knadh/jsonconfig"
)

func main() {
	// setup the structure
	config := struct {
		Url string `json:"url"`

		Methods []string `json:"methods"`

		AlwaysLoad  bool `json:"always_load"`

		Module struct{
			Name string `json:"name"`
			Route string `json:"route"`
			Port int `json:"port"`
		} `json:"module"`
	}{}

	// parse and load json config
	err := jsonconfig.Load("config.json", &config)

	if err == nil {
		println("The url is", config.Url)
		println("Supported methods are", config.Methods[0], config.Methods[1])
		
		println("The module is", config.Module.Name, "on route", config.Module.Route)
	}
}
```