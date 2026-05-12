package aws

import (
	"github.com/aws/aws-sdk-go/service/elbv2"
	log "github.com/sirupsen/logrus"

	"github.com/Qovery/pleco/pkg/common"
)

func DeleteOrphanedTargetGroups(sessions AWSSessions, options AwsOptions) {
	region := *sessions.ELB.Config.Region

	orphaned, err := listOrphanedTargetGroups(sessions.ELB)
	if err != nil {
		log.Errorf("Can't list orphaned target groups in %s: %s", region, err)
		return
	}

	count, start := common.ElemToDeleteFormattedInfos("orphaned ELB target group", len(orphaned), region)
	log.Info(count)

	if options.DryRun || len(orphaned) == 0 {
		return
	}

	log.Info(start)
	deleteTargetGroups(sessions.ELB, orphaned)
}

func listOrphanedTargetGroups(lbSession *elbv2.ELBV2) ([]*elbv2.TargetGroup, error) {
	var orphaned []*elbv2.TargetGroup

	err := lbSession.DescribeTargetGroupsPages(&elbv2.DescribeTargetGroupsInput{}, func(page *elbv2.DescribeTargetGroupsOutput, _ bool) bool {
		for _, tg := range page.TargetGroups {
			if len(tg.LoadBalancerArns) == 0 {
				orphaned = append(orphaned, tg)
			}
		}
		return true
	})

	return orphaned, err
}

func deleteTargetGroups(lbSession *elbv2.ELBV2, tgs []*elbv2.TargetGroup) {
	for _, tg := range tgs {
		_, err := lbSession.DeleteTargetGroup(&elbv2.DeleteTargetGroupInput{
			TargetGroupArn: tg.TargetGroupArn,
		})
		if err != nil {
			log.Errorf("Can't delete target group %s in %s: %s", *tg.TargetGroupName, *lbSession.Config.Region, err)
		} else {
			log.Debugf("Target group %s deleted in %s.", *tg.TargetGroupName, *lbSession.Config.Region)
		}
	}
}
