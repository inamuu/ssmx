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

func ListSessionTargets(ctx context.Context, cfg aws.Config) ([]SessionTarget, error) {
	instances, err := ListRunningInstances(ctx, cfg)
	if err != nil {
		return nil, err
	}

	targets := make([]SessionTarget, 0, len(instances))
	for _, inst := range instances {
		targets = append(targets, SessionTarget{
			Kind:      SessionTargetKindEC2,
			TargetID:  inst.InstanceID,
			Name:      inst.Name,
			PrivateIP: inst.PrivateIP,
		})
	}

	ecsTargets, err := ListExecTargets(ctx, cfg)
	if err != nil {
		return nil, err
	}

	targets = append(targets, ecsTargets...)
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
