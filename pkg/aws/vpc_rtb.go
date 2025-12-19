package aws

import (
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	log "github.com/sirupsen/logrus"

	"github.com/Qovery/pleco/pkg/common"
)

type RouteTable struct {
	Id           string
	CreationDate time.Time
	ttl          int64
	Associations []*ec2.RouteTableAssociation
	IsProtected  bool
}

func getRouteTablesByVpcId(ec2Session *ec2.EC2, vpcId string) []*ec2.RouteTable {
	input := &ec2.DescribeRouteTablesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []*string{aws.String(vpcId)},
			},
		},
	}

	routeTables, err := ec2Session.DescribeRouteTables(input)
	if err != nil {
		log.Error(err)
	}

	return routeTables.RouteTables
}

func GetRouteTablesIdsByVpcId(ec2Session *ec2.EC2, vpcId string, tagName string) []RouteTable {
	var routeTablesStruct []RouteTable

	routeTables := getRouteTablesByVpcId(ec2Session, vpcId)

	for _, routeTable := range routeTables {
		essentialTags := common.GetEssentialTags(routeTable.Tags, tagName)

		var routeTableStruct = RouteTable{
			Id:           *routeTable.RouteTableId,
			CreationDate: essentialTags.CreationDate,
			ttl:          essentialTags.TTL,
			Associations: routeTable.Associations,
			IsProtected:  essentialTags.IsProtected,
		}
		routeTablesStruct = append(routeTablesStruct, routeTableStruct)
	}

	return routeTablesStruct
}

func disassociateRouteTable(ec2Session *ec2.EC2, routeTable RouteTable) {
	// Disassociate all non-main associations
	for _, association := range routeTable.Associations {
		if association.RouteTableAssociationId == nil {
			continue
		}

		// Skip main associations as they cannot be disassociated
		if association.Main != nil && *association.Main {
			continue
		}

		_, err := ec2Session.DisassociateRouteTable(&ec2.DisassociateRouteTableInput{
			AssociationId: association.RouteTableAssociationId,
		})

		if err != nil {
			log.Errorf("Failed to disassociate route table %s from association %s: %s", routeTable.Id, *association.RouteTableAssociationId, err.Error())
		} else {
			log.Debugf("Route table %s disassociated from %s in %s.", routeTable.Id, *association.RouteTableAssociationId, *ec2Session.Config.Region)
		}
	}
}

func deleteRoutesFromRouteTable(ec2Session *ec2.EC2, routeTableId string) {
	// Describe the route table to get all routes
	result, err := ec2Session.DescribeRouteTables(&ec2.DescribeRouteTablesInput{
		RouteTableIds: []*string{aws.String(routeTableId)},
	})

	if err != nil {
		log.Errorf("Failed to describe route table %s: %s", routeTableId, err.Error())
		return
	}

	if len(result.RouteTables) == 0 {
		return
	}

	routeTable := result.RouteTables[0]

	// Delete all non-local routes
	for _, route := range routeTable.Routes {
		// Skip local routes as they cannot be deleted manually
		if route.GatewayId != nil && *route.GatewayId == "local" {
			continue
		}

		// Skip routes without a destination (shouldn't happen but being defensive)
		if route.DestinationCidrBlock == nil && route.DestinationIpv6CidrBlock == nil && route.DestinationPrefixListId == nil {
			continue
		}

		// Build delete route input based on which destination type is present
		deleteInput := &ec2.DeleteRouteInput{
			RouteTableId: aws.String(routeTableId),
		}

		if route.DestinationCidrBlock != nil {
			deleteInput.DestinationCidrBlock = route.DestinationCidrBlock
		} else if route.DestinationIpv6CidrBlock != nil {
			deleteInput.DestinationIpv6CidrBlock = route.DestinationIpv6CidrBlock
		} else if route.DestinationPrefixListId != nil {
			deleteInput.DestinationPrefixListId = route.DestinationPrefixListId
		}

		_, deleteErr := ec2Session.DeleteRoute(deleteInput)
		if deleteErr != nil {
			log.Errorf("Failed to delete route from route table %s: %s", routeTableId, deleteErr.Error())
		} else {
			destination := ""
			if route.DestinationCidrBlock != nil {
				destination = *route.DestinationCidrBlock
			} else if route.DestinationIpv6CidrBlock != nil {
				destination = *route.DestinationIpv6CidrBlock
			} else if route.DestinationPrefixListId != nil {
				destination = *route.DestinationPrefixListId
			}
			log.Debugf("Route %s from route table %s in %s deleted.", destination, routeTableId, *ec2Session.Config.Region)
		}
	}
}

func DeleteRouteTablesByIds(ec2Session *ec2.EC2, routeTables []RouteTable) {
	for _, routeTable := range routeTables {
		if !isMainRouteTable(routeTable) && !routeTable.IsProtected {
			// Disassociate route table from subnets first
			disassociateRouteTable(ec2Session, routeTable)

			// Delete all non-local routes to avoid dependency violations
			deleteRoutesFromRouteTable(ec2Session, routeTable.Id)

			_, err := ec2Session.DeleteRouteTable(
				&ec2.DeleteRouteTableInput{
					RouteTableId: aws.String(routeTable.Id),
				},
			)

			if err != nil {
				log.Error(err)
			} else {
				log.Debugf("Route table %s in %s deleted.", routeTable.Id, *ec2Session.Config.Region)
			}
		}
	}
}

func isMainRouteTable(routeTable RouteTable) bool {
	for _, association := range routeTable.Associations {
		if *association.Main && routeTable.Id == *association.RouteTableId {
			return true
		}
	}

	return false
}
