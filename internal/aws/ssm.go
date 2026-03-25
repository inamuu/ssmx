package aws

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

const commandPollInterval = 2 * time.Second

func ListManagedInstances(ctx context.Context, cfg aws.Config) ([]Instance, error) {
	ssmClient := ssm.NewFromConfig(cfg)
	instanceIDs, err := listManagedInstanceIDs(ctx, ssmClient)
	if err != nil {
		return nil, err
	}
	if len(instanceIDs) == 0 {
		return nil, nil
	}

	ec2Client := ec2.NewFromConfig(cfg)
	output, err := ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: instanceIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("describe instances: %w", err)
	}

	var instances []Instance
	for _, reservation := range output.Reservations {
		for _, inst := range reservation.Instances {
			instances = append(instances, Instance{
				InstanceID: aws.ToString(inst.InstanceId),
				PrivateIP:  aws.ToString(inst.PrivateIpAddress),
				Name:       extractNameTag(inst.Tags),
			})
		}
	}

	sort.Slice(instances, func(i, j int) bool {
		return instances[i].InstanceID < instances[j].InstanceID
	})

	return instances, nil
}

func SendCommand(ctx context.Context, cfg aws.Config, instanceID, documentName, workDir string, commands []string) (string, error) {
	client := ssm.NewFromConfig(cfg)

	params := map[string][]string{
		"commands": commands,
	}
	if workDir != "" {
		params["workingDirectory"] = []string{workDir}
	}

	output, err := client.SendCommand(ctx, &ssm.SendCommandInput{
		DocumentName: aws.String(documentName),
		InstanceIds:  []string{instanceID},
		Parameters:   params,
	})
	if err != nil {
		return "", err
	}

	return aws.ToString(output.Command.CommandId), nil
}

func WaitForCommandInvocation(ctx context.Context, cfg aws.Config, commandID, instanceID string) (*ssm.GetCommandInvocationOutput, error) {
	client := ssm.NewFromConfig(cfg)

	waiter := ssm.NewCommandExecutedWaiter(client, func(o *ssm.CommandExecutedWaiterOptions) {
		o.MinDelay = commandPollInterval
		o.MaxDelay = 10 * time.Second
	})

	input := &ssm.GetCommandInvocationInput{
		CommandId:  aws.String(commandID),
		InstanceId: aws.String(instanceID),
	}

	waitErr := waiter.Wait(ctx, input, timeoutRemaining(ctx))
	output, getErr := client.GetCommandInvocation(ctx, input)
	if getErr != nil {
		if waitErr != nil {
			return nil, fmt.Errorf("wait for command result: %w (get invocation: %v)", waitErr, getErr)
		}
		return nil, fmt.Errorf("get command invocation: %w", getErr)
	}

	if waitErr != nil && output.Status == ssmtypes.CommandInvocationStatusInProgress {
		return nil, fmt.Errorf("wait for command result: %w", waitErr)
	}

	return output, nil
}

func listManagedInstanceIDs(ctx context.Context, client *ssm.Client) ([]string, error) {
	paginator := ssm.NewDescribeInstanceInformationPaginator(client, &ssm.DescribeInstanceInformationInput{
		Filters: []ssmtypes.InstanceInformationStringFilter{
			{
				Key:    aws.String("PingStatus"),
				Values: []string{"Online"},
			},
			{
				Key:    aws.String("ResourceType"),
				Values: []string{"EC2Instance"},
			},
		},
	})

	var instanceIDs []string
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("describe managed instances: %w", err)
		}
		for _, info := range page.InstanceInformationList {
			if id := aws.ToString(info.InstanceId); id != "" {
				instanceIDs = append(instanceIDs, id)
			}
		}
	}

	return instanceIDs, nil
}

func timeoutRemaining(ctx context.Context) time.Duration {
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining > 0 {
			return remaining
		}
	}
	return time.Second
}
