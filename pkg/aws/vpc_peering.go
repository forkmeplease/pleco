package aws

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	log "github.com/sirupsen/logrus"
)

func DeleteVpcPeeringConnectionsByVpcId(ec2Session *ec2.EC2, vpcId string) {
	// Find all VPC peering connections for this VPC (both accepter and requester)
	input := &ec2.DescribeVpcPeeringConnectionsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("requester-vpc-info.vpc-id"),
				Values: []*string{aws.String(vpcId)},
			},
		},
	}

	result, err := ec2Session.DescribeVpcPeeringConnections(input)
	if err != nil {
		log.Errorf("Failed to describe VPC peering connections for VPC %s: %s", vpcId, err.Error())
		return
	}

	// Also check for accepter VPC
	inputAccepter := &ec2.DescribeVpcPeeringConnectionsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("accepter-vpc-info.vpc-id"),
				Values: []*string{aws.String(vpcId)},
			},
		},
	}

	resultAccepter, err := ec2Session.DescribeVpcPeeringConnections(inputAccepter)
	if err != nil {
		log.Errorf("Failed to describe VPC peering connections (accepter) for VPC %s: %s", vpcId, err.Error())
	} else {
		result.VpcPeeringConnections = append(result.VpcPeeringConnections, resultAccepter.VpcPeeringConnections...)
	}

	if len(result.VpcPeeringConnections) == 0 {
		log.Debugf("No VPC peering connections found for VPC %s", vpcId)
		return
	}

	// Delete each VPC peering connection
	for _, peering := range result.VpcPeeringConnections {
		if peering.VpcPeeringConnectionId == nil {
			continue
		}

		// Skip if already deleted or rejected
		if peering.Status != nil && peering.Status.Code != nil {
			status := *peering.Status.Code
			if status == "deleted" || status == "rejected" || status == "deleting" {
				log.Debugf("Skipping VPC peering connection %s (status: %s)", *peering.VpcPeeringConnectionId, status)
				continue
			}
		}

		_, deleteErr := ec2Session.DeleteVpcPeeringConnection(&ec2.DeleteVpcPeeringConnectionInput{
			VpcPeeringConnectionId: peering.VpcPeeringConnectionId,
		})

		if deleteErr != nil {
			log.Errorf("Failed to delete VPC peering connection %s: %s", *peering.VpcPeeringConnectionId, deleteErr.Error())
		} else {
			log.Debugf("VPC peering connection %s in %s deleted.", *peering.VpcPeeringConnectionId, *ec2Session.Config.Region)
		}
	}
}
