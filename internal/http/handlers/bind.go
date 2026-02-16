package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

type FieldError struct {
	Field   string `json:"field"`
	Rule    string `json:"rule"`
	Param   string `json:"param,omitempty"`
	Message string `json:"message,omitempty"`
}

func BindJSON(ctx *gin.Context, out interface{}) bool {
	err := ctx.ShouldBindJSON(out)

	if err != nil {
		RespondBadRequest(ctx, "Invalid request body", parseBindError(err, out))

		return false
	}

	return true
}

func parseBindError(err error, out interface{}) interface{} {
	rootType := baseStructType(out)

	// validator errors (struct bind tags)

	var validatorError validator.ValidationErrors

	if errors.As(err, &validatorError) {
		fields := make([]FieldError, 0, len(validatorError))

		for _, fieldError := range validatorError {
			field := jsonPathFromValidatorError(rootType, fieldError)
			rule := fieldError.Tag()
			param := fieldError.Param()

			fields = append(fields, FieldError{
				Field:   field,
				Rule:    rule,
				Param:   param,
				Message: validationMessage(rule, param),
			})
		}
		return gin.H{"fields": fields}
	}

	// in the event of bad json

	var syntaxError *json.SyntaxError

	if errors.As(err, &syntaxError) {
		return gin.H{
			"json": "invalid_json_syntax",
		}
	}

	// in the event of a type mismatch

	var unmatchedTypeError *json.UnmarshalTypeError

	if errors.As(err, &unmatchedTypeError) {
		field := jsonPathFromDotPath(rootType, unmatchedTypeError.Field)

		if field == "" {
			field = strings.TrimSpace(unmatchedTypeError.Field)
		}

		return gin.H{
			"json":  "invalid_json_type",
			"field": field,
			"fields": []FieldError{
				{
					Field:   field,
					Rule:    "type",
					Message: fmt.Sprintf("must be of type %s", unmatchedTypeError.Type.String()),
				},
			},
		}
	}

	// final fallback if the error could not be deciphered
	return gin.H{"reason": err.Error()}
}

func baseStructType(v interface{}) reflect.Type {
	t := reflect.TypeOf(v)

	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	if t != nil && t.Kind() == reflect.Struct {
		return t
	}

	return nil
}

func jsonPathFromValidatorError(rootType reflect.Type, fieldError validator.FieldError) string {
	// Namespace format is usually "<StructName>.<Field>[.<NestedField>...]".
	namespace := fieldError.StructNamespace()
	if namespace == "" {
		namespace = fieldError.Namespace()
	}

	if namespace == "" {
		return fieldError.Field()
	}

	parts := strings.Split(namespace, ".")
	if len(parts) == 0 {
		return fieldError.Field()
	}

	if rootType != nil && rootType.Name() != "" && parts[0] == rootType.Name() {
		parts = parts[1:]
	}

	path := mapStructPathToJSONPath(rootType, parts)
	if path != "" {
		return path
	}

	return fieldError.Field()
}

func jsonPathFromDotPath(rootType reflect.Type, dotPath string) string {
	dotPath = strings.TrimSpace(dotPath)
	if dotPath == "" {
		return ""
	}

	return mapStructPathToJSONPath(rootType, strings.Split(dotPath, "."))
}

func mapStructPathToJSONPath(rootType reflect.Type, parts []string) string {
	if len(parts) == 0 {
		return ""
	}

	current := rootType
	out := make([]string, 0, len(parts))

	for _, rawPart := range parts {
		if rawPart == "" {
			continue
		}

		fieldName, indexSuffix := splitFieldIndex(rawPart)
		jsonName := fieldName

		nextType := reflect.Type(nil)
		if current != nil {
			for current.Kind() == reflect.Pointer {
				current = current.Elem()
			}

			if current.Kind() == reflect.Struct {
				if sf, ok := current.FieldByName(fieldName); ok {
					jsonName = jsonNameFromStructField(sf)
					nextType = sf.Type
				}
			}
		}

		out = append(out, jsonName+indexSuffix)

		if nextType != nil {
			current = unwindCollection(nextType)
		} else {
			current = nil
		}
	}

	return strings.Join(out, ".")
}

func splitFieldIndex(part string) (string, string) {
	idx := strings.Index(part, "[")
	if idx == -1 {
		return part, ""
	}

	return part[:idx], part[idx:]
}

func jsonNameFromStructField(sf reflect.StructField) string {
	tag := sf.Tag.Get("json")
	if tag == "" {
		return sf.Name
	}

	name, _, _ := strings.Cut(tag, ",")
	if name == "" || name == "-" {
		return sf.Name
	}

	return name
}

func unwindCollection(t reflect.Type) reflect.Type {
	for t != nil {
		switch t.Kind() {
		case reflect.Pointer, reflect.Slice, reflect.Array:
			t = t.Elem()
		default:
			return t
		}
	}

	return nil
}

func validationMessage(rule, param string) string {
	switch rule {
	case "required":
		return "is required"
	case "email":
		return "must be a valid email address"
	case "min":
		return "must be at least " + param
	case "max":
		return "must be at most " + param
	case "len":
		return "must be exactly " + param
	case "oneof":
		return "must be one of " + strings.ReplaceAll(param, " ", ", ")
	default:
		if param != "" {
			return fmt.Sprintf("failed %s validation (%s)", rule, param)
		}
		return "failed " + rule + " validation"
	}
}
