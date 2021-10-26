package deadmanssnitchintegration

import (
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
)

type mockEC2 struct {
	ec2iface.EC2API
}

func NewMockEC2() *mockEC2 {
	return &mockEC2{}
}

func (c *mockEC2) DescribeTags(input *ec2.DescribeTagsInput) (*ec2.DescribeTagsOutput, error) {
	key := "test1"
	value := "test2"
	dto := &ec2.DescribeTagsOutput{
		Tags: []*ec2.TagDescription{
			{
				Key:   &key,
				Value: &value,
			},
		},
	}
	return dto, nil
}

func (c *mockEC2) DescribeInstances(input *ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error) {
	name := "running"
	dio := &ec2.DescribeInstancesOutput{
		Reservations: []*ec2.Reservation{
			{
				Instances: []*ec2.Instance{
					{
						State: &ec2.InstanceState{
							Name: &name,
						},
					},
				},
			},
		},
	}
	return dio, nil
}
