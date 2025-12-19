package aws

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	log "github.com/sirupsen/logrus"
)

func DeleteVpcEndpointsByVpcId(ec2Session *ec2.EC2, vpcId string) {
	// Find all VPC endpoints for this VPC
	input := &ec2.DescribeVpcEndpointsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []*string{aws.String(vpcId)},
			},
		},
	}

	result, err := ec2Session.DescribeVpcEndpoints(input)
	if err != nil {
		log.Errorf("Failed to describe VPC endpoints for VPC %s: %s", vpcId, err.Error())
		return
	}

	if len(result.VpcEndpoints) == 0 {
		log.Debugf("No VPC endpoints found for VPC %s", vpcId)
		return
	}

	// Delete each VPC endpoint
	for _, endpoint := range result.VpcEndpoints {
		if endpoint.VpcEndpointId == nil {
			continue
		}

		_, deleteErr := ec2Session.DeleteVpcEndpoints(&ec2.DeleteVpcEndpointsInput{
			VpcEndpointIds: []*string{endpoint.VpcEndpointId},
		})

		if deleteErr != nil {
			log.Errorf("Failed to delete VPC endpoint %s: %s", *endpoint.VpcEndpointId, deleteErr.Error())
		} else {
			log.Debugf("VPC endpoint %s in %s deleted.", *endpoint.VpcEndpointId, *ec2Session.Config.Region)
		}
	}
}
