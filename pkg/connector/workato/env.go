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
		return "", uhttp.WrapErrors(codes.InvalidArgument, fmt.Sprintf("baton-workato invalid environment '%s', must be one of: prod, test, dev", env))
	}
}
