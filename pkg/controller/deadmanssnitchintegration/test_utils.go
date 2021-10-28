package deadmanssnitchintegration

import (
	"github.com/aws/aws-sdk-go/service/cloudtrail"
	"github.com/aws/aws-sdk-go/service/cloudtrail/cloudtrailiface"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
)

type mockEC2 struct {
	instanceState *string
	ec2iface.EC2API
}

func NewMockEC2(instanceState string) *mockEC2 {
	return &mockEC2{
		instanceState: &instanceState,
	}
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
	instanceId := "i-abcdefgh"
	tagKey := "Name"
	tagValue := "testClusterName-1234-master-whatever"
	dio := &ec2.DescribeInstancesOutput{
		Reservations: []*ec2.Reservation{
			{
				Instances: []*ec2.Instance{
					{
						InstanceId: &instanceId,
						State: &ec2.InstanceState{
							Name: c.instanceState,
						},
						Tags: []*ec2.Tag{
							{
								Key:   &tagKey,
								Value: &tagValue,
							},
						},
					},
				},
			},
		},
	}
	return dio, nil
}

type mockCT struct {
	eventName     *string
	eventUsername *string
	cloudtrailiface.CloudTrailAPI
}

func NewMockCT(eventName string, eventUsername string) *mockCT {
	return &mockCT{
		eventName:     &eventName,
		eventUsername: &eventUsername,
	}
}

func (c *mockCT) LookupEvents(input *cloudtrail.LookupEventsInput) (*cloudtrail.LookupEventsOutput, error) {
	leo := &cloudtrail.LookupEventsOutput{
		Events: []*cloudtrail.Event{
			{
				EventName: c.eventName,
				Username:  c.eventUsername,
			},
		},
	}
	return leo, nil
}
