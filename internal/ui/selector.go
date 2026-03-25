package ui

import (
	"fmt"

	internalaws "github.com/inamuu/ssmx/internal/aws"
	fuzzyfinder "github.com/ktr0731/go-fuzzyfinder"
)

func SelectInstance(instances []internalaws.Instance) (*internalaws.Instance, error) {
	idx, err := fuzzyfinder.Find(
		instances,
		func(i int) string {
			inst := instances[i]
			return fmt.Sprintf("%-22s  %-16s  %s", inst.InstanceID, inst.PrivateIP, inst.Name)
		},
		fuzzyfinder.WithPreviewWindow(func(i, w, h int) string {
			if i == -1 {
				return ""
			}
			inst := instances[i]
			return fmt.Sprintf(
				"Instance ID: %s\nPrivate IP:  %s\nName:        %s",
				inst.InstanceID,
				inst.PrivateIP,
				inst.Name,
			)
		}),
	)
	if err != nil {
		return nil, err
	}
	return &instances[idx], nil
}
