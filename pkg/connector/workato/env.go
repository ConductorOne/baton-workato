package workato

import (
	"fmt"

	"github.com/conductorone/baton-sdk/pkg/uhttp"
	"google.golang.org/grpc/codes"
)

type Environment string

func (e Environment) String() string {
	return string(e)
}

var (
	Production  Environment = "prod"
	Test        Environment = "test"
	Development Environment = "dev"
	// All is used internally to sync all environments. It is not a valid workato environment.
	All Environment = "all"
)

func EnvFromString(env string) (Environment, error) {
	switch env {
	case Production.String():
		return Production, nil
	case Test.String():
		return Test, nil
	case Development.String():
		return Development, nil
	case All.String():
		return All, nil
	default:
		return "", uhttp.WrapErrors(codes.InvalidArgument, fmt.Sprintf("baton-workato invalid environment '%s', must be one of: prod, test, dev or all", env))
	}
}

func EnvironmentDisplayName(env Environment) (string, error) {
	switch env {
	case Production:
		return "Production", nil
	case Test:
		return "Test", nil
	case Development:
		return "Development", nil
	default:
		return "", fmt.Errorf("invalid environment '%s'", env)
	}
}

func AllEnvironments() []Environment {
	return []Environment{
		Production,
		Test,
		Development,
	}
}
