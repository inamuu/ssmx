package session

import (
	"context"
	"errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	plugindatachannel "github.com/aws/session-manager-plugin/src/datachannel"
	pluginlog "github.com/aws/session-manager-plugin/src/log"
	pluginsession "github.com/aws/session-manager-plugin/src/sessionmanagerplugin/session"
	_ "github.com/aws/session-manager-plugin/src/sessionmanagerplugin/session/portsession"
	_ "github.com/aws/session-manager-plugin/src/sessionmanagerplugin/session/shellsession"
	"github.com/google/uuid"
	internalaws "github.com/inamuu/ssmx/internal/aws"
)

func startECSPluginSession(ctx context.Context, cfg aws.Config, target internalaws.SessionTarget) error {
	output, err := ecs.NewFromConfig(cfg).ExecuteCommand(ctx, &ecs.ExecuteCommandInput{
		Cluster:     aws.String(target.ClusterName),
		Task:        aws.String(target.TaskArn),
		Container:   aws.String(target.ContainerName),
		Command:     aws.String(target.Command),
		Interactive: true,
	})
	if err != nil {
		return err
	}
	if output.Session == nil || output.Session.SessionId == nil || output.Session.StreamUrl == nil || output.Session.TokenValue == nil {
		return errors.New("execute-command response missing session details")
	}

	s := &pluginsession.Session{
		SessionId:   aws.ToString(output.Session.SessionId),
		StreamUrl:   aws.ToString(output.Session.StreamUrl),
		TokenValue:  aws.ToString(output.Session.TokenValue),
		Endpoint:    "",
		ClientId:    uuid.NewString(),
		TargetId:    target.PrimaryLabel(),
		Region:      cfg.Region,
		DataChannel: &plugindatachannel.DataChannel{},
	}

	return s.Execute(pluginlog.Logger(false, s.ClientId))
}
