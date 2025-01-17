package workato

import "errors"

type Environment string

func (e Environment) String() string {
	return string(e)
}

var (
	Production  Environment = "prod"
	Test        Environment = "test"
	Development Environment = "dev"
)

func EnvFromString(env string) (Environment, error) {
	switch env {
	case Production.String():
		return Production, nil
	case Test.String():
		return Test, nil
	case Development.String():
		return Development, nil
	default:
		return "", errors.New("invalid environment")
	}
}
