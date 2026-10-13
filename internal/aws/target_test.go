package aws

import "testing"

func TestParseTargetSpec(t *testing.T) {
	got, err := ParseTargetSpec("i-0123456789abcdef0")
	if err != nil || got.Kind != SessionTargetKindEC2 || got.TargetID != "i-0123456789abcdef0" {
		t.Fatalf("EC2 spec: got %+v, err %v", got, err)
	}

	got, err = ParseTargetSpec("ecs:my-cluster/abc123/app")
	if err != nil {
		t.Fatalf("ECS spec: %v", err)
	}
	if got.Kind != SessionTargetKindECS || got.ClusterName != "my-cluster" || got.TaskArn != "abc123" || got.ContainerName != "app" {
		t.Fatalf("ECS spec: got %+v", got)
	}

	for _, bad := range []string{"ecs:", "ecs:a/b", "ecs:a//c", "ecs:a/b/c/d"} {
		if _, err := ParseTargetSpec(bad); err == nil {
			t.Fatalf("ParseTargetSpec(%q) should fail", bad)
		}
	}
}

func TestParseTargetKind(t *testing.T) {
	cases := map[string]SessionTargetKind{"": "", "ec2": SessionTargetKindEC2, "ECS": SessionTargetKindECS, "container": SessionTargetKindECS}
	for in, want := range cases {
		got, err := ParseTargetKind(in)
		if err != nil || got != want {
			t.Fatalf("ParseTargetKind(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := ParseTargetKind("lambda"); err == nil {
		t.Fatal("ParseTargetKind(lambda) should fail")
	}
}
