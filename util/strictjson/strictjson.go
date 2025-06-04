package strictjson

import (
	"encoding/json"
	"fmt"
)

// Unmarshal unmarshals JSON data into the provided struct v while ensuring strict validation of the input.
//
// It checks for any extra or missing fields in the JSON data that do not match the struct definition.
//
// Unlike [json.Unmarshal], Unmarshal is case-sensitive.
//
// Example usage:
//
//	type MyStruct struct {
//	    Field1 string `json:"field1"`
//	    Field2 int    `json:"field2"`
//	}
//
//	var obj MyStruct
//	err := Unmarshal([]byte(`{"field1": "value", "field2": 42}`), &obj)
//	if err != nil {
//	    log.Fatal(err)
//	}
func Unmarshal(data []byte, v any) error {
	// Marshal the struct to JSON
	marshaledStruct, err := json.Marshal(v)
	if err != nil {
		return err
	}

	var structKeyMap map[string]any
	if err := json.Unmarshal(marshaledStruct, &structKeyMap); err != nil {
		return err
	}

	// Unmarshal the JSON into a map
	var jsonKeyMap map[string]any
	if err := json.Unmarshal(data, &jsonKeyMap); err != nil {
		return err
	}

	// Check for extra field in JSON input
	for key := range jsonKeyMap {
		if _, exists := structKeyMap[key]; !exists {
			return fmt.Errorf("Unexpected field: %s", key)
		}
	}

	// Check for missing fields in JSON input
	for key := range structKeyMap {
		if _, exists := jsonKeyMap[key]; !exists {
			return fmt.Errorf("Missing field: %s", key)
		}
	}

	// Finally store JSON into struct v
	err = json.Unmarshal(data, v)
	if err != nil {
		return err
	}

	return nil
}
