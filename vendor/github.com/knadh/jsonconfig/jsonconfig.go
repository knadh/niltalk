// Package jsonconfig is a super tiny (pseudo) JSON configuration parser for Go with support for comments.

// Kailash Nadh, http://nadh.in/code/jsonconfig
// Jan 2015
// MIT License

package jsonconfig

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"regexp"
)

func Load(filename string, config interface{}) error {
	// Read the config file.
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return errors.New("Error reading file")
	}

	// Regex monstrosity because of the lack of lookbehinds/aheads.

	// Standalone comments.
	r1, _ := regexp.Compile(`(?m)^(\s+)?//(.*)$`)

	// Numbers and boolean.
	r2, _ := regexp.Compile(`(?m)"(.+?)":(\s+)?([0-9\.\-]+|true|false|null)(\s+)?,(\s+)?//(.*)$`)

	// Strings.
	r3, _ := regexp.Compile(`(?m)"(.+?)":(\s+)?"(.+?)"(\s+)?,(\s+)?//(.*)$`)

	// Arrays and objects.
	r4, _ := regexp.Compile(`(?m)"(.+?)":(\s+)?([\{\[])(.+?)([\}\]])(\s+)?,(\s+)?//(.*)$`)

	res := r1.ReplaceAllString(string(data), "")
	res = r2.ReplaceAllString(res, `"$1": $3,`)
	res = r3.ReplaceAllString(res, `"$1": "$3",`)
	res = r4.ReplaceAllString(res, `"$1": $3$4$5,`)

	// Decode json.
	if err := json.Unmarshal([]byte(res), &config); err != nil {
		return err
	}

	return nil
}
