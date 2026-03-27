package aws

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"

	sdkaws "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

const ecsExecShellCommand = "/bin/sh"

func ListExecTargets(ctx context.Context, cfg sdkaws.Config) ([]SessionTarget, error) {
	client := ecs.NewFromConfig(cfg)

	clusterPaginator := ecs.NewListClustersPaginator(client, &ecs.ListClustersInput{})

	var targets []SessionTarget
	for clusterPaginator.HasMorePages() {
		clusterPage, err := clusterPaginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("list ECS clusters: %w", err)
		}

		for _, clusterArn := range clusterPage.ClusterArns {
			clusterName := path.Base(clusterArn)

			taskArns, err := listRunningTaskARNs(ctx, client, clusterArn)
			if err != nil {
				return nil, err
			}
			if len(taskArns) == 0 {
				continue
			}

			clusterTargets, err := describeExecTargets(ctx, client, clusterArn, clusterName, taskArns)
			if err != nil {
				return nil, err
			}
			targets = append(targets, clusterTargets...)
		}
	}

	sort.Slice(targets, func(i, j int) bool {
		if targets[i].ClusterName != targets[j].ClusterName {
			return targets[i].ClusterName < targets[j].ClusterName
		}
		if targets[i].TaskID != targets[j].TaskID {
			return targets[i].TaskID < targets[j].TaskID
		}
		return targets[i].ContainerName < targets[j].ContainerName
	})

	return targets, nil
}

func listRunningTaskARNs(ctx context.Context, client *ecs.Client, clusterArn string) ([]string, error) {
	paginator := ecs.NewListTasksPaginator(client, &ecs.ListTasksInput{
		Cluster:       sdkaws.String(clusterArn),
		DesiredStatus: ecstypes.DesiredStatusRunning,
	})

	var taskArns []string
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("list ECS tasks for %s: %w", clusterArn, err)
		}
		taskArns = append(taskArns, page.TaskArns...)
	}

	return taskArns, nil
}

func describeExecTargets(ctx context.Context, client *ecs.Client, clusterArn, clusterName string, taskArns []string) ([]SessionTarget, error) {
	const batchSize = 100

	var targets []SessionTarget
	for start := 0; start < len(taskArns); start += batchSize {
		end := start + batchSize
		if end > len(taskArns) {
			end = len(taskArns)
		}

		output, err := client.DescribeTasks(ctx, &ecs.DescribeTasksInput{
			Cluster: sdkaws.String(clusterArn),
			Tasks:   taskArns[start:end],
		})
		if err != nil {
			return nil, fmt.Errorf("describe ECS tasks for %s: %w", clusterArn, err)
		}

		for _, task := range output.Tasks {
			if !task.EnableExecuteCommand {
				continue
			}

			taskID := path.Base(sdkaws.ToString(task.TaskArn))
			taskName := deriveTaskName(task)

			for _, container := range task.Containers {
				if !isExecReady(container) {
					continue
				}

				targets = append(targets, SessionTarget{
					Kind:          SessionTargetKindECS,
					TargetID:      sdkaws.ToString(task.TaskArn),
					Name:          taskName,
					ClusterName:   clusterName,
					TaskArn:       sdkaws.ToString(task.TaskArn),
					TaskID:        taskID,
					ContainerName: sdkaws.ToString(container.Name),
					Command:       ecsExecShellCommand,
				})
			}
		}
	}

	return targets, nil
}

func isExecReady(container ecstypes.Container) bool {
	if sdkaws.ToString(container.RuntimeId) == "" {
		return false
	}

	for _, agent := range container.ManagedAgents {
		if string(agent.Name) == "ExecuteCommandAgent" && strings.EqualFold(sdkaws.ToString(agent.LastStatus), "RUNNING") {
			return true
		}
	}

	return false
}

func deriveTaskName(task ecstypes.Task) string {
	if group := sdkaws.ToString(task.Group); strings.HasPrefix(group, "service:") {
		return strings.TrimPrefix(group, "service:")
	}
	if family := sdkaws.ToString(task.TaskDefinitionArn); family != "" {
		return path.Base(family)
	}
	return ""
}
