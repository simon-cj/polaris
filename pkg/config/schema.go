package config

import (
	"encoding/json"
	"fmt"

	"github.com/qri-io/jsonschema"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

type resourceMinimum string
type resourceMaximum string

func newResourceMinimum() jsonschema.Validator {
	return new(resourceMinimum)
}

func newResourceMaximum() jsonschema.Validator {
	return new(resourceMaximum)
}

func (min resourceMinimum) Validate(path string, data interface{}, errs *[]jsonschema.ValError) {
	err := validateRange(path, string(min), data, true)
	if err != nil {
		*errs = append(*errs, *err...)
	}
}

func (max resourceMaximum) Validate(path string, data interface{}, errs *[]jsonschema.ValError) {
	err := validateRange(path, string(max), data, false)
	if err != nil {
		*errs = append(*errs, *err...)
	}
}

func parseQuantity(i interface{}) (resource.Quantity, *[]jsonschema.ValError) {
	resStr, ok := i.(string)
	if !ok {
		return resource.Quantity{}, &[]jsonschema.ValError{
			{Message: fmt.Sprintf("Resource quantity %v is not a string", i)},
		}
	}
	q, err := resource.ParseQuantity(resStr)
	if err != nil {
		return resource.Quantity{}, &[]jsonschema.ValError{
			{Message: fmt.Sprintf("Could not parse resource quantity: %s", resStr)},
		}
	}
	return q, nil
}

func validateRange(path string, limit interface{}, data interface{}, isMinimum bool) *[]jsonschema.ValError {
	limitQuantity, err := parseQuantity(limit)
	if err != nil {
		return err
	}
	actualQuantity, err := parseQuantity(data)
	if err != nil {
		return err
	}
	cmp := limitQuantity.Cmp(actualQuantity)
	if isMinimum {
		if cmp == 1 {
			return &[]jsonschema.ValError{
				{Message: fmt.Sprintf("%s quantity %v is > %v", path, actualQuantity, limitQuantity)},
			}
		}
	} else {
		if cmp == -1 {
			return &[]jsonschema.ValError{
				{Message: fmt.Sprintf("%s quantity %v is < %v", path, actualQuantity, limitQuantity)},
			}
		}
	}
	return nil
}

func init() {
	jsonschema.RegisterValidator("resourceMinimum", newResourceMinimum)
	jsonschema.RegisterValidator("resourceMaximum", newResourceMaximum)
}

type includeExcludeList struct {
	Include []string `yaml:"include"`
	Exclude []string `yaml:"exclude"`
}

type TargetKind string

const (
	TargetContainer TargetKind = "Container"
	TargetPod       TargetKind = "Pod"
)

// SchemaCheck is a Polaris check that runs using JSON Schema
type SchemaCheck struct {
	Name           string                `yaml:"name"`
	ID             string                `yaml:"id"`
	Category       string                `yaml:"category"`
	SuccessMessage string                `yaml:"successMessage"`
	FailureMessage string                `yaml:"failureMessage"`
	Controllers    includeExcludeList    `yaml:"controllers"`
	Containers     includeExcludeList    `yaml:"containers"`
	Target         TargetKind            `yaml:"target"`
	SchemaTarget   TargetKind            `yaml:"schemaTarget"`
	Schema         jsonschema.RootSchema `yaml:"schema"`
}

func (check SchemaCheck) CheckPod(pod *corev1.PodSpec) (bool, error) {
	return check.CheckObject(pod)
}

func (check SchemaCheck) CheckContainer(container *corev1.Container) (bool, error) {
	return check.CheckObject(container)
}

func (check SchemaCheck) CheckObject(obj interface{}) (bool, error) {
	bytes, err := json.Marshal(obj)
	if err != nil {
		return false, err
	}
	errs, err := check.Schema.ValidateBytes(bytes)
	return len(errs) == 0, err
}

func (check SchemaCheck) IsActionable(target TargetKind, controllerType SupportedController, isInit bool) bool {
	if check.Target != target {
		return false
	}
	isIncluded := len(check.Controllers.Include) == 0
	for _, inclusion := range check.Controllers.Include {
		if GetSupportedControllerFromString(inclusion) == controllerType {
			isIncluded = true
			break
		}
	}
	if !isIncluded {
		return false
	}
	for _, exclusion := range check.Controllers.Exclude {
		if GetSupportedControllerFromString(exclusion) == controllerType {
			return false
		}
	}
	if check.Target == TargetContainer {
		isIncluded := len(check.Containers.Include) == 0
		for _, inclusion := range check.Containers.Include {
			if (inclusion == "initContainer" && isInit) || (inclusion == "container" && !isInit) {
				isIncluded = true
				break
			}
		}
		if !isIncluded {
			return false
		}
		for _, exclusion := range check.Containers.Exclude {
			if (exclusion == "initContainer" && isInit) || (exclusion == "container" && !isInit) {
				return false
			}
		}
	}
	return true
}
