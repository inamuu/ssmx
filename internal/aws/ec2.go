package aws

import (
	"context"
	"sort"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

type Instance struct {
	InstanceID string
	PrivateIP  string
	Name       string
}

func ListRunningInstances(ctx context.Context, cfg aws.Config) ([]Instance, error) {
	client := ec2.NewFromConfig(cfg)

	input := &ec2.DescribeInstancesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("instance-state-name"),
				Values: []string{"running"},
			},
		},
	}

	var instances []Instance
	paginator := ec2.NewDescribeInstancesPaginator(client, input)
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, reservation := range output.Reservations {
			for _, inst := range reservation.Instances {
				instances = append(instances, Instance{
					InstanceID: aws.ToString(inst.InstanceId),
					PrivateIP:  aws.ToString(inst.PrivateIpAddress),
					Name:       extractNameTag(inst.Tags),
				})
			}
		}
	}

	sort.Slice(instances, func(i, j int) bool {
		return instances[i].InstanceID < instances[j].InstanceID
	})

	return instances, nil
}

// ListSessionTargets returns running EC2 instances and ECS exec targets.
// kind limits the result to one kind; an empty kind returns both.
func ListSessionTargets(ctx context.Context, cfg aws.Config, kind SessionTargetKind) ([]SessionTarget, error) {
	return listTargets(ctx, cfg, kind, ListRunningInstances)
}

// ListCopyTargets returns SSM-managed (Online) EC2 instances and ECS exec
// targets, i.e. targets that can receive a file without an interactive shell.
func ListCopyTargets(ctx context.Context, cfg aws.Config, kind SessionTargetKind) ([]SessionTarget, error) {
	return listTargets(ctx, cfg, kind, ListManagedInstances)
}

func listTargets(ctx context.Context, cfg aws.Config, kind SessionTargetKind, listInstances func(context.Context, aws.Config) ([]Instance, error)) ([]SessionTarget, error) {
	var targets []SessionTarget

	if kind == "" || kind == SessionTargetKindEC2 {
		instances, err := listInstances(ctx, cfg)
		if err != nil {
			return nil, err
		}
		for _, inst := range instances {
			targets = append(targets, SessionTarget{
				Kind:      SessionTargetKindEC2,
				TargetID:  inst.InstanceID,
				Name:      inst.Name,
				PrivateIP: inst.PrivateIP,
			})
		}
	}

	if kind == "" || kind == SessionTargetKindECS {
		ecsTargets, err := ListExecTargets(ctx, cfg)
		if err != nil {
			return nil, err
		}
		targets = append(targets, ecsTargets...)
	}

	return targets, nil
}

func extractNameTag(tags []types.Tag) string {
	for _, tag := range tags {
		if aws.ToString(tag.Key) == "Name" {
			return aws.ToString(tag.Value)
		}
	}
	return ""
}
