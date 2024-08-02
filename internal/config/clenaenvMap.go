package config

// As cleanenv doesn't really like our nested map[string]Project, we're applying
// Missing parts of the functionality through this helper functions.
// Those helpers using reflect to get struct-tag data and currently only support
// string data in config, as it's the only type of data we need.

import (
	"fmt"
	"os"
	"reflect"
)

// env values are not applied to the nested map/slice, so we're applying them
// manually
func applyEnvToProjectAndActions(cfg *Config) {
	for projectName, project := range cfg.Projects {

		projectPrefix := `PROJECTS__` + projectName + "__"

		envStructFields := getEnvStructFields(&project)
		for _, envValues := range envStructFields {
			if value, ok := os.LookupEnv(projectPrefix + envValues.envKey); ok {
				setEnvValue(envValues.value, value)
			}
		}

		for i, action := range project.Actions {
			actionPrefix := projectPrefix + fmt.Sprintf("ACTIONS__%d__", i+1)

			for _, envValues := range getEnvStructFields(&action) {
				if value, ok := os.LookupEnv(actionPrefix + envValues.envKey); ok {
					setEnvValue(envValues.value, value)
				}
			}
			project.Actions[i] = action
		}

		cfg.Projects[projectName] = project
	}
}

func setEnvValue(fieldValue reflect.Value, str string) {
	switch fieldValue.Kind() {
	case reflect.String:
		fieldValue.SetString(str)
		// no other type is currently supported
	}
}

type StructEnvFields struct {
	value  reflect.Value
	envKey string
}

// getEnvStructFields retrieves struct field values, that are marked with
// env string with the corresponding value of the env struct tag.
func getEnvStructFields[T Project | Action](item *T) []StructEnvFields {
	arr := make([]StructEnvFields, 0)
	itemType := reflect.TypeOf(*item)
	itemValue := reflect.ValueOf(item).Elem()
	for i := 0; i < itemType.NumField(); i++ {
		field := itemType.Field(i)
		value := itemValue.Field(i)
		key := field.Tag.Get("env")
		if key != "" {
			arr = append(arr, StructEnvFields{value, key})
		}
	}
	return arr
}

// setDefaultAndCheckRequired sets Project or Action default values from the struct tags
// and raises an error if some of the required fields are not supplied
func setDefaultAndCheckRequired[T Project | Action](item *T) error {
	typesType := reflect.TypeOf(*item)
	typesValue := reflect.ValueOf(item).Elem()
	for i := 0; i < typesType.NumField(); i++ {
		field := typesType.Field(i)
		fieldValue := typesValue.Field(i)
		isRequired := field.Tag.Get("env-required") == "true"

		// Only string type
		if fieldValue.Type().Kind() == reflect.String && fieldValue.String() == "" {
			defaultValue := field.Tag.Get("env-default")
			if defaultValue != "" {
				fieldValue.SetString(defaultValue)
			} else if isRequired {
				return fmt.Errorf("required field '%s' is missing", field.Name)
			}
		}
	}
	return nil
}
