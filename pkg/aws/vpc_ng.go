package aws

import (
	"time"

	"github.com/Qovery/pleco/pkg/common"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	log "github.com/sirupsen/logrus"
)

type NatGateway struct {
	common.CloudProviderResource
}

func getNatGatewaysByVpcId(ec2Session *ec2.EC2, options *AwsOptions, vpcId string) []NatGateway {
	input := &ec2.DescribeNatGatewaysInput{
		Filter: []*ec2.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []*string{aws.String(vpcId)},
			},
		},
	}

	gateways, err := ec2Session.DescribeNatGateways(input)
	if err != nil {
		log.Error(err)
	}

	return gtwResponseToStruct(gateways.NatGateways, options.TagName)
}

func getNatGateways(ec2Session *ec2.EC2, tagName string) []NatGateway {
	gateways, err := ec2Session.DescribeNatGateways(&ec2.DescribeNatGatewaysInput{})
	if err != nil {
		log.Error(err)
	}

	return gtwResponseToStruct(gateways.NatGateways, tagName)
}

func getExpiredNatGateways(ec2Session *ec2.EC2, options *AwsOptions) []NatGateway {
	gateways := getNatGateways(ec2Session, options.TagName)

	expiredGtws := []NatGateway{}
	for _, gtw := range gateways {
		if gtw.IsResourceExpired(options.TagValue, options.DisableTTLCheck) {
			expiredGtws = append(expiredGtws, gtw)
		}
	}

	return expiredGtws
}

func GetNatGatewaysIdsByVpcId(ec2Session *ec2.EC2, options *AwsOptions, vpcId string) []NatGateway {
	return getNatGatewaysByVpcId(ec2Session, options, vpcId)
}

func DeleteNatGatewaysByIds(ec2Session *ec2.EC2, natGateways []NatGateway) {
	var deletedGatewayIds []*string

	for _, natGateway := range natGateways {
		if !natGateway.IsProtected {

			_, deleteErr := ec2Session.DeleteNatGateway(
				&ec2.DeleteNatGatewayInput{
					NatGatewayId: aws.String(natGateway.Identifier),
				},
			)

			if deleteErr != nil {
				log.Error(deleteErr)
			} else {
				log.Debugf("Nat Gateway %s in %s deletion initiated.", natGateway.Identifier, *ec2Session.Config.Region)
				deletedGatewayIds = append(deletedGatewayIds, aws.String(natGateway.Identifier))
			}
		}
	}

	// Wait for NAT Gateways to be fully deleted
	if len(deletedGatewayIds) > 0 {
		log.Debugf("Waiting for %d NAT Gateway(s) to be deleted...", len(deletedGatewayIds))
		waitForNatGatewayDeletion(ec2Session, deletedGatewayIds)
	}
}

func waitForNatGatewayDeletion(ec2Session *ec2.EC2, natGatewayIds []*string) {
	maxWaitTime := 10 * time.Minute
	checkInterval := 15 * time.Second
	startTime := time.Now()

	for {
		if time.Since(startTime) > maxWaitTime {
			log.Warnf("Timeout waiting for NAT Gateways to be deleted after %v", maxWaitTime)
			return
		}

		result, err := ec2Session.DescribeNatGateways(&ec2.DescribeNatGatewaysInput{
			NatGatewayIds: natGatewayIds,
		})

		if err != nil {
			log.Debugf("Error describing NAT Gateways (might be deleted): %s", err.Error())
			return
		}

		allDeleted := true
		for _, ng := range result.NatGateways {
			if ng.State != nil && *ng.State != "deleted" {
				allDeleted = false
				log.Debugf("NAT Gateway %s status: %s", *ng.NatGatewayId, *ng.State)
				break
			}
		}

		if allDeleted || len(result.NatGateways) == 0 {
			log.Debugf("All NAT Gateways deleted successfully")
			return
		}

		time.Sleep(checkInterval)
	}
}

func gtwResponseToStruct(result []*ec2.NatGateway, tagName string) []NatGateway {
	gtws := []NatGateway{}
	for _, key := range result {
		essentialTags := common.GetEssentialTags(key.Tags, tagName)
		gtw := NatGateway{
			CloudProviderResource: common.CloudProviderResource{
				Identifier:   *key.NatGatewayId,
				Description:  "Nat Gateway: " + *key.NatGatewayId,
				CreationDate: key.CreateTime.UTC(),
				TTL:          essentialTags.TTL,
				Tag:          essentialTags.Tag,
				IsProtected:  essentialTags.IsProtected,
			},
		}

		gtws = append(gtws, gtw)
	}

	return gtws
}

func DeleteExpiredNatGateways(sessions AWSSessions, options AwsOptions) {
	gtws := getExpiredNatGateways(sessions.EC2, &options)
	region := sessions.EC2.Config.Region

	count, start := common.ElemToDeleteFormattedInfos("expired Nat Gateway", len(gtws), *region)

	log.Info(count)

	if options.DryRun || len(gtws) == 0 {
		return
	}

	log.Info(start)

	DeleteNatGatewaysByIds(sessions.EC2, gtws)

}
