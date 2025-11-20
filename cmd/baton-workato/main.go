package main

import (
	"context"
	"fmt"
	"os"

	"github.com/conductorone/baton-workato/pkg/connector/workato"

	"github.com/conductorone/baton-workato/pkg/connector/client"

	"github.com/conductorone/baton-sdk/pkg/config"
	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	"github.com/conductorone/baton-sdk/pkg/field"
	"github.com/conductorone/baton-sdk/pkg/types"
	cfg "github.com/conductorone/baton-workato/pkg/config"
	"github.com/conductorone/baton-workato/pkg/connector"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
)

var version = "dev"

func main() {
	ctx := context.Background()

	_, cmd, err := config.DefineConfiguration(
		ctx,
		"baton-workato",
		getConnector,
		cfg.Config,
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	cmd.Version = version

	err = cmd.Execute()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func getConnector(ctx context.Context, wc *cfg.Workato) (types.ConnectorServer, error) {
	l := ctxzap.Extract(ctx)
	err := field.Validate(cfg.Config, wc)
	if err != nil {
		return nil, err
	}
	
	dataCenterUrl := client.WorkatoDataCenters[wc.WorkatoDataCenter]

	env, err := workato.EnvFromString(wc.WorkatoEnv)
	if err != nil {
		return nil, err
	}

	workatoClient, err := client.NewWorkatoClient(ctx, wc.WorkatoApiKey, dataCenterUrl)
	if err != nil {
		return nil, err
	}

	cb, err := connector.New(ctx, workatoClient, env, wc.DisableCustomRolesSync)
	if err != nil {
		l.Error("error creating connector", zap.Error(err))
		return nil, err
	}
	connector, err := connectorbuilder.NewConnector(ctx, cb)
	if err != nil {
		l.Error("error creating connector", zap.Error(err))
		return nil, err
	}
	return connector, nil
}
