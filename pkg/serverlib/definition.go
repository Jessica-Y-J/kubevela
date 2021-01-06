package serverlib

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"

	mycue "github.com/oam-dev/kubevela/pkg/cue"
	"github.com/oam-dev/kubevela/pkg/utils/common"
	"github.com/oam-dev/kubevela/pkg/utils/system"
)

const (
	// OpenAPISchemaDir is the folder name under ~/.vela/capabilities
	OpenAPISchemaDir = "openapi"
	// UsageTag is usage comment annotation
	UsageTag = "+usage="
	// ShortTag is the short alias annotation
	ShortTag = "\\n+short"
)

// OpenAPISchema is the struct for OpenAPI Schema generated by Cue OpenAPI
type OpenAPISchema struct {
	OpenAPI    string     `json:"openapi"`
	Components Components `json:"components"`
}

// Components is the struct filed of OpenAPISchema
type Components struct {
	Schemas Schemas `json:"schemas"`
}

// Schemas is the struct filed of Components
type Schemas struct {
	Parameter map[string]interface{} `json:"parameter"`
}

// GetDefinition is the main function for GetDefinition API
func GetDefinition(name string) ([]byte, error) {
	openAPISchema, err := generateOpenAPISchemaFromCapabilityParameter(name)
	if err != nil {
		return nil, err
	}
	swagger, err := openapi3.NewSwaggerLoader().LoadSwaggerFromData(openAPISchema)
	if err != nil {
		return nil, err
	}
	schemaRef := swagger.Components.Schemas["parameter"]
	schema := schemaRef.Value
	fixOpenAPISchema("", schema)

	parameter, err := schema.MarshalJSON()
	if err != nil {
		return nil, err
	}
	return parameter, nil
}

// generateOpenAPISchemaFromCapabilityParameter returns the parameter of a definition in cue.Value format
func generateOpenAPISchemaFromCapabilityParameter(name string) ([]byte, error) {
	dir, err := system.GetCapabilityDir()
	if err != nil {
		return nil, err
	}

	definitionCueName := fmt.Sprintf("%s.cue", name)
	schemaDir := filepath.Join(dir, OpenAPISchemaDir)
	if err = prepareParameterCue(dir, definitionCueName, schemaDir); err != nil {
		return nil, err
	}

	if err = appendCueReference(filepath.Join(dir, OpenAPISchemaDir, definitionCueName)); err != nil {
		return nil, err
	}

	filename := filepath.FromSlash(definitionCueName)
	return common.GenOpenAPIFromFile(filepath.Join(dir, OpenAPISchemaDir), filename)
}

// prepareParameterCue cuts `parameter` section form definition .cue file
func prepareParameterCue(fileDir, fileName string, targetSchemaDir string) error {
	if _, err := os.Stat(targetSchemaDir); err != nil && os.IsNotExist(err) {
		if err := os.Mkdir(targetSchemaDir, 0750); err != nil {
			return err
		}
	}

	cueFile := filepath.Join(fileDir, fileName)
	f, err := os.Open(filepath.Clean(cueFile))
	if err != nil {
		return err
	}
	//nolint
	defer f.Close()
	schemaFile := filepath.Join(targetSchemaDir, fileName)
	_, err = os.Stat(schemaFile)
	if err == nil {
		if err = os.Truncate(schemaFile, 0); err != nil {
			return err
		}
	}
	targetFile, err := os.OpenFile(filepath.Clean(schemaFile), os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	//nolint
	defer targetFile.Close()

	scanner := bufio.NewScanner(f)
	var withParameterFlag bool
	r := regexp.MustCompile("[[:space:]]*parameter:[[:space:]]*{.*")

	for scanner.Scan() {
		text := scanner.Text()
		if r.MatchString(text) {
			// a variable has to be refined as a definition which starts with "#"
			text = fmt.Sprintf("parameter: #parameter\n#%s", text)
			withParameterFlag = true
		}
		if _, err := targetFile.WriteString(fmt.Sprintf("%s\n", text)); err != nil {
			return err
		}
	}

	if !withParameterFlag {
		return fmt.Errorf("cue file %s doesn't contain section `parmeter`", cueFile)
	}
	return nil
}

// appendCueReference appends `context` filed to parameter .cue file
func appendCueReference(cueFile string) error {
	f, err := os.OpenFile(filepath.Clean(cueFile), os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	//nolint
	defer f.Close()

	if _, err := f.WriteString(mycue.BaseTemplate); err != nil {
		return err
	}
	return nil
}

// fixOpenAPISchema fixes tainted `description` filed, missing of title `field`.
func fixOpenAPISchema(name string, schema *openapi3.Schema) {
	t := schema.Type
	switch t {
	case "object":
		for k, v := range schema.Properties {
			s := v.Value
			fixOpenAPISchema(k, s)
		}
	case "array":
		if name != "" {
			schema.Title = name
		}
		fixOpenAPISchema("", schema.Items.Value)
	default:
		if name != "" {
			schema.Title = name
		}
	}
	line := schema.Description
	if strings.Contains(line, UsageTag) {
		newDescription := strings.ReplaceAll(line, UsageTag, "")
		if strings.Contains(newDescription, ShortTag) {
			newDescription = strings.ReplaceAll(newDescription, ShortTag, "")
		}
		schema.Description = newDescription
	}
}

// addTitleField adds title field
func addTitleField(line, propertyName string) string {
	blanks := strings.Split(line, "\"")[0]
	title := fmt.Sprintf("%s\"title\": \"%s\"", blanks, propertyName)
	return title + ",\n"
}

// getParameterItemName gets the name of a parameter item
func getParameterItemName(previousLine string) (string, error) {
	// parse property name, like from `"cmd": {\n`
	tempPropertyName := strings.Split(previousLine, "\"")
	if len(tempPropertyName) <= 1 {
		return "", fmt.Errorf("could not get property name from: %s", previousLine)
	}
	propertyName := strings.Split(tempPropertyName[1], "\"")[0]
	return propertyName, nil
}

// getParameterFromOpenAPISchema retrieves needs section from OpenAPI schema for front-end requirements
func getParameterFromOpenAPISchema(openAPISchema []byte) ([]byte, error) {
	var schema OpenAPISchema
	var parameterJSON []byte
	var err error
	if err = json.Unmarshal(openAPISchema, &schema); err != nil {
		return nil, err
	}

	parameter := schema.Components.Schemas.Parameter
	if parameterJSON, err = json.Marshal(parameter); err != nil {
		return nil, err
	}
	return parameterJSON, nil
}
