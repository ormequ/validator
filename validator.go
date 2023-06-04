package govalid

import (
	"github.com/pkg/errors"
	"reflect"
	"strconv"
	"strings"
)

var ErrNotStruct = errors.New("wrong argument given, should be a struct")
var ErrInvalidValidatorSyntax = errors.New("invalid validator syntax")
var ErrValidateForUnexportedFields = errors.New("validation for unexported field is not allowed")

type ValidationError struct {
	Err error
}

type ValidationErrors []ValidationError

func (v ValidationErrors) Error() string {
	s := make([]string, len(v))
	for i := range v {
		s[i] += v[i].Err.Error()
	}
	return strings.Join(s, "; ")
}

func Validate(s any) error {
	val := reflect.ValueOf(s)
	if val.Kind() != reflect.Struct {
		return ErrNotStruct
	}
	valErrs, e := ValidateStruct(val)
	if e != nil {
		return e
	}
	if len(valErrs) == 0 {
		return nil
	}
	return valErrs
}

// ValidateField validates a single field in struct. It used checkT as a type what is needed to check
// and checkV as a value (or values separated by comma) to check
func ValidateField(v reflect.Value, name string, checkT string, checkV string) ValidationError {
	var err ValidationError
	if checkT == "min" || checkT == "max" || checkT == "len" {
		size := getSize(v)
		err = validateSize(size, checkT, checkV, name+" has incorrect size")
	} else if checkT == "in" {
		errText := name + " has a value that is not present in 'validate'"
		if len(checkV) == 0 {
			return ValidationError{ErrInvalidValidatorSyntax}
		}
		set := strings.Split(checkV, ",")
		for j := range set {
			switch v.Kind() {
			case reflect.Int:
				err = validateSize(int(v.Int()), "len", set[j], errText)
			case reflect.String:
				if set[j] != v.String() {
					err.Err = errors.New(errText)
				} else {
					err.Err = nil
				}
			}
			if err.Err == nil || err.Err.Error() != errText {
				break
			}
		}
	}
	return err
}

// ValidateStruct validates all fields in struct s
func ValidateStruct(s reflect.Value) (ValidationErrors, error) {
	if s.Kind() != reflect.Struct {
		return nil, errors.New("Validating value is not struct")
	}
	valErrs := make(ValidationErrors, 0)
	for i := 0; i < s.NumField(); i++ {
		field := s.Type().Field(i)
		// Nested validating
		if field.Type.Kind() == reflect.Struct && field.IsExported() {
			nested, err := ValidateStruct(s.Field(i))
			if err != nil {
				return nil, err
			}
			valErrs = append(valErrs, nested...)
			continue
		}

		fullTag, ok := field.Tag.Lookup("validate")
		if !ok {
			continue
		}
		if !field.IsExported() {
			valErrs = append(valErrs, ValidationError{ErrValidateForUnexportedFields})
			continue
		}
		// Validate data is separated by ;
		splitTag := strings.Split(fullTag, ";")
		for _, tag := range splitTag {
			tagInfo := strings.SplitN(strings.TrimLeft(tag, " "), ":", 2)
			if len(tagInfo) != 2 {
				valErrs = append(valErrs, ValidationError{ErrInvalidValidatorSyntax})
				continue
			}
			v := s.Field(i)
			var err ValidationError
			if v.Kind() == reflect.Slice {
				// For slice find first invalid value
				for j := 0; j < v.Len(); j++ {
					err = ValidateField(v.Index(j), field.Name, tagInfo[0], tagInfo[1])
					if err.Err != nil {
						break
					}
				}
			} else {
				err = ValidateField(v, field.Name, tagInfo[0], tagInfo[1])
			}
			if err.Err != nil {
				valErrs = append(valErrs, err)
				break
			}
		}
	}
	return valErrs, nil
}

// getSize returns size of value on this rule: int -> returns v, string -> returns len(v)
func getSize(v reflect.Value) int {
	switch v.Kind() {
	case reflect.Int:
		return int(v.Int())
	case reflect.String:
		return len(v.String())
	}
	return -1
}

// validateSize checks whether sz meet the requirements in t with "check" as comparable value.
// If not will be returned error with text invalidText
func validateSize(sz int, t string, check string, invalidText string) ValidationError {
	valid := true
	n, err := strconv.Atoi(check)
	if err != nil {
		return ValidationError{ErrInvalidValidatorSyntax}
	}
	switch t {
	case "max":
		valid = sz <= n
	case "min":
		valid = sz >= n
	case "len":
		valid = sz == n
	}
	if !valid {
		return ValidationError{
			errors.New(invalidText),
		}
	}
	return ValidationError{}
}
