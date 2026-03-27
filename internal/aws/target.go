package aws

import "fmt"

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
