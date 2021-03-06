// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2017 Canonical Ltd
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License version 3 as
 * published by the Free Software Foundation.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package config

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var validKey = regexp.MustCompile("^(?:[a-z0-9]+-?)*[a-z](?:-?[a-z0-9])*$")

func ParseKey(key string) (subkeys []string, err error) {
	subkeys = strings.Split(key, ".")
	for _, subkey := range subkeys {
		if !validKey.MatchString(subkey) {
			return nil, fmt.Errorf("invalid option name: %q", subkey)
		}
	}
	return subkeys, nil
}

func PatchConfig(snapName string, subkeys []string, pos int, config interface{}, value *json.RawMessage) (interface{}, error) {

	switch config := config.(type) {
	case nil:
		// Missing update map. Create and nest final value under it.
		configm := make(map[string]interface{})
		_, err := PatchConfig(snapName, subkeys, pos, configm, value)
		if err != nil {
			return nil, err
		}
		return configm, nil

	case *json.RawMessage:
		// Raw replaces pristine on commit. Unpack, update, and repack.
		var configm map[string]interface{}
		err := json.Unmarshal([]byte(*config), &configm)
		if err != nil {
			return nil, fmt.Errorf("snap %q option %q is not a map", snapName, strings.Join(subkeys[:pos], "."))
		}
		_, err = PatchConfig(snapName, subkeys, pos, configm, value)
		if err != nil {
			return nil, err
		}
		return jsonRaw(configm), nil

	case map[string]interface{}:
		// Update map to apply against pristine on commit.
		if pos+1 == len(subkeys) {
			config[subkeys[pos]] = value
			return config, nil
		} else {
			result, err := PatchConfig(snapName, subkeys, pos+1, config[subkeys[pos]], value)
			if err != nil {
				return nil, err
			}
			config[subkeys[pos]] = result
			return config, nil
		}
	}
	panic(fmt.Errorf("internal error: unexpected configuration type %T", config))
}

// Get unmarshals into result the value of the provided snap's configuration key.
// If the key does not exist, an error of type *NoOptionError is returned.
// The provided key may be formed as a dotted key path through nested maps.
// For example, the "a.b.c" key describes the {a: {b: {c: value}}} map.
func GetFromChange(snapName string, subkeys []string, pos int, config map[string]interface{}, result interface{}) error {
	value, ok := config[subkeys[pos]]
	if !ok {
		return &NoOptionError{SnapName: snapName, Key: strings.Join(subkeys[:pos+1], ".")}
	}

	if pos+1 == len(subkeys) {
		raw, ok := value.(*json.RawMessage)
		if !ok {
			raw = jsonRaw(value)
		}
		err := json.Unmarshal([]byte(*raw), result)
		if err != nil {
			key := strings.Join(subkeys, ".")
			return fmt.Errorf("internal error: cannot unmarshal snap %q option %q into %T: %s, json: %s", snapName, key, result, err, *raw)
		}
		return nil
	}

	configm, ok := value.(map[string]interface{})
	if !ok {
		raw, ok := value.(*json.RawMessage)
		if !ok {
			raw = jsonRaw(value)
		}
		err := json.Unmarshal([]byte(*raw), &configm)
		if err != nil {
			return fmt.Errorf("snap %q option %q is not a map", snapName, strings.Join(subkeys[:pos+1], "."))
		}
	}
	return GetFromChange(snapName, subkeys, pos+1, configm, result)
}
