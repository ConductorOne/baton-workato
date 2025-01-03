package main

import (
	"context"
	"fmt"
	"github.com/conductorone/baton-workato/pkg/connector/workato"
	"os"

	"github.com/conductorone/baton-workato/cmd/baton-workato/conf"

	"github.com/conductorone/baton-workato/pkg/connector/client"

	"github.com/conductorone/baton-sdk/pkg/config"
	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	"github.com/conductorone/baton-sdk/pkg/field"
	"github.com/conductorone/baton-sdk/pkg/types"
	"github.com/conductorone/baton-workato/pkg/connector"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var version = "dev"

func main() {
	ctx := context.Background()

	_, cmd, err := config.DefineConfiguration(
		ctx,
		"baton-workato",
		getConnector,
		field.Configuration{
			Fields: conf.ConfigurationFields,
		},
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

func getConnector(ctx context.Context, v *viper.Viper) (types.ConnectorServer, error) {
	l := ctxzap.Extract(ctx)
	if err := conf.ValidateConfig(v); err != nil {
		return nil, err
	}

	key := v.GetString(conf.ApiKeyField.FieldName)
	dataCenterUrl := client.WorkatoDataCenters[v.GetString(conf.WorkatoDataCenterFiekd.FieldName)]

	env, err := workato.EnvFromString(v.GetString(conf.WorkatoEnv.FieldName))
	if err != nil {
		return nil, err
	}

	workatoClient, err := client.NewWorkatoClient(ctx, key, dataCenterUrl)
	if err != nil {
		return nil, err
	}

	cb, err := connector.New(ctx, workatoClient, env)
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
