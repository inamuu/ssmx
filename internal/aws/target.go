package aws

import (
	"fmt"
	"strings"
)

type SessionTargetKind string

const (
	SessionTargetKindEC2 SessionTargetKind = "ec2"
	SessionTargetKindECS SessionTargetKind = "ecs"
)

type SessionTarget struct {
	Kind          SessionTargetKind
	TargetID      string
	Name          string
	PrivateIP     string
	ClusterName   string
	TaskArn       string
	TaskID        string
	ContainerName string
	Command       string
}

func (t SessionTarget) PrimaryLabel() string {
	switch t.Kind {
	case SessionTargetKindECS:
		return fmt.Sprintf("ecs:%s/%s/%s", t.ClusterName, t.TaskID, t.ContainerName)
	default:
		return t.TargetID
	}
}

func (t SessionTarget) SecondaryLabel() string {
	switch t.Kind {
	case SessionTargetKindECS:
		if t.Name != "" {
			return t.Name
		}
		return t.ClusterName
	default:
		return t.PrivateIP
	}
}

func (t SessionTarget) DetailText() string {
	switch t.Kind {
	case SessionTargetKindECS:
		return fmt.Sprintf(
			"Type:        ECS\nContainer:   %s\nTask:        %s\nCluster:     %s\nCommand:     %s\nName:        %s",
			t.ContainerName,
			t.TaskID,
			t.ClusterName,
			t.Command,
			t.Name,
		)
	default:
		return fmt.Sprintf(
			"Type:        EC2\nInstance ID: %s\nPrivate IP:  %s\nName:        %s",
			t.TargetID,
			t.PrivateIP,
			t.Name,
		)
	}
}

func (t SessionTarget) ErrorLabel() string {
	switch t.Kind {
	case SessionTargetKindECS:
		return fmt.Sprintf("cluster=%s task=%s container=%s", t.ClusterName, t.TaskID, t.ContainerName)
	default:
		return fmt.Sprintf("instance=%s", t.TargetID)
	}
}

// ParseTargetSpec converts a --target value into a SessionTarget.
// "ecs:<cluster>/<task-id>/<container>" selects an ECS container, anything
// else is treated as an EC2 instance ID.
func ParseTargetSpec(spec string) (SessionTarget, error) {
	if !strings.HasPrefix(spec, "ecs:") {
		return SessionTarget{Kind: SessionTargetKindEC2, TargetID: spec}, nil
	}

	parts := strings.Split(strings.TrimPrefix(spec, "ecs:"), "/")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return SessionTarget{}, fmt.Errorf("invalid ECS target %q (want ecs:<cluster>/<task-id>/<container>)", spec)
	}

	return SessionTarget{
		Kind:          SessionTargetKindECS,
		TargetID:      parts[1],
		ClusterName:   parts[0],
		TaskArn:       parts[1],
		TaskID:        parts[1],
		ContainerName: parts[2],
		Command:       ecsExecShellCommand,
	}, nil
}

// ParseTargetKind validates a --type value. An empty value means all kinds.
func ParseTargetKind(s string) (SessionTargetKind, error) {
	switch strings.ToLower(s) {
	case "":
		return "", nil
	case "ec2", "instance":
		return SessionTargetKindEC2, nil
	case "ecs", "container":
		return SessionTargetKindECS, nil
	default:
		return "", fmt.Errorf("invalid --type %q (want ec2 or ecs)", s)
	}
}
