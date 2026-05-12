package aws

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	log "github.com/sirupsen/logrus"

	"github.com/Qovery/pleco/pkg/common"
)

func DeleteOrphanedLBCSecurityGroups(sessions AWSSessions, options AwsOptions) {
	region := *sessions.EC2.Config.Region

	orphaned, err := listOrphanedLBCSecurityGroups(sessions.EC2)
	if err != nil {
		log.Errorf("Can't list orphaned LBC security groups in %s: %s", region, err)
		return
	}

	count, start := common.ElemToDeleteFormattedInfos("orphaned LBC security group", len(orphaned), region)
	log.Info(count)

	if options.DryRun || len(orphaned) == 0 {
		return
	}

	log.Info(start)
	DeleteSecurityGroupsByIds(sessions.EC2, orphaned)
}

func listOrphanedLBCSecurityGroups(ec2Session *ec2.EC2) ([]SecurityGroup, error) {
	// Collect all SG IDs currently referenced by an ENI.
	usedSGIds := map[string]struct{}{}
	err := ec2Session.DescribeNetworkInterfacesPages(
		&ec2.DescribeNetworkInterfacesInput{},
		func(page *ec2.DescribeNetworkInterfacesOutput, _ bool) bool {
			for _, eni := range page.NetworkInterfaces {
				for _, g := range eni.Groups {
					usedSGIds[aws.StringValue(g.GroupId)] = struct{}{}
				}
			}
			return true
		},
	)
	if err != nil {
		return nil, err
	}

	// Find k8s-* SGs not referenced by any ENI.
	var orphaned []SecurityGroup
	err = ec2Session.DescribeSecurityGroupsPages(
		&ec2.DescribeSecurityGroupsInput{
			Filters: []*ec2.Filter{
				{Name: aws.String("group-name"), Values: []*string{aws.String("k8s-*")}},
			},
		},
		func(page *ec2.DescribeSecurityGroupsOutput, _ bool) bool {
			for _, sg := range page.SecurityGroups {
				if _, inUse := usedSGIds[aws.StringValue(sg.GroupId)]; !inUse {
					orphaned = append(orphaned, SecurityGroup{
						Id:                  aws.StringValue(sg.GroupId),
						IpPermissionIngress: sg.IpPermissions,
						IpPermissionEgress:  sg.IpPermissionsEgress,
					})
				}
			}
			return true
		},
	)

	return orphaned, err
}
