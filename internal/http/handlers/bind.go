package handlers

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

type FieldError struct {
	Field string `json:"field"`
	Rule string `json:"rule"`
}


func BindJSON(ctx *gin.Context, out interface{} )bool {
	err := ctx.ShouldBindJSON(out)

	if err != nil {
		RespondBadRequest(ctx,"Invalid request body", parseBindError(err))

		return false
	}

	return true
}


func parseBindError(err error) interface{} {
	// validator errors (struct bind tags)

	var validatorError validator.ValidationErrors

	if errors.As(err,&validatorError){
		out := make([]FieldError, 0,len(validatorError))

		for _, field_error := range validatorError {
			out  = append(out, FieldError{
				Field: strings.ToLower(field_error.Field()),
				Rule: field_error.Tag(),
			})
		}
		return gin.H{"fields":out}
	}
	
	// in the event of bad json

	var syntax_error *json.SyntaxError

	if errors.As(err,&syntax_error) {
		return gin.H{
			"json": "invalid_json_syntax",
		}
	}

	// in the event of a type mismatch

	var unmatchedTypeError *json.UnmarshalTypeError

	if errors.As(err,&unmatchedTypeError) {
		return gin.H{
			"json": "invalid_json_type",
			"field": unmatchedTypeError.Field,
		}
	}

	// final fallback if the error could not be deciphered
	return gin.H{"reason": err.Error()}
}