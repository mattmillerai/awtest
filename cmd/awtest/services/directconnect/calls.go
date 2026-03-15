package directconnect

import (
	"context"
	"fmt"
	"time"

	"github.com/MillerMedia/awtest/cmd/awtest/types"
	"github.com/MillerMedia/awtest/cmd/awtest/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/directconnect"
)

type dcConnection struct {
	ConnectionId    string
	ConnectionName  string
	ConnectionState string
	Bandwidth       string
	Location        string
	OwnerAccount    string
	PartnerName     string
	Region          string
}

type dcVirtualInterface struct {
	VirtualInterfaceId    string
	VirtualInterfaceName  string
	VirtualInterfaceState string
	VirtualInterfaceType  string
	ConnectionId          string
	Vlan                  string
	Asn                   string
	AmazonAddress         string
	CustomerAddress       string
	Region                string
}

type dcGateway struct {
	DirectConnectGatewayId    string
	DirectConnectGatewayName  string
	DirectConnectGatewayState string
	AmazonSideAsn             string
	OwnerAccount              string
}

func extractConnection(conn *directconnect.Connection, region string) dcConnection {
	connId := ""
	if conn.ConnectionId != nil {
		connId = *conn.ConnectionId
	}
	name := ""
	if conn.ConnectionName != nil {
		name = *conn.ConnectionName
	}
	state := ""
	if conn.ConnectionState != nil {
		state = *conn.ConnectionState
	}
	bandwidth := ""
	if conn.Bandwidth != nil {
		bandwidth = *conn.Bandwidth
	}
	location := ""
	if conn.Location != nil {
		location = *conn.Location
	}
	ownerAccount := ""
	if conn.OwnerAccount != nil {
		ownerAccount = *conn.OwnerAccount
	}
	partnerName := ""
	if conn.PartnerName != nil {
		partnerName = *conn.PartnerName
	}
	return dcConnection{
		ConnectionId:    connId,
		ConnectionName:  name,
		ConnectionState: state,
		Bandwidth:       bandwidth,
		Location:        location,
		OwnerAccount:    ownerAccount,
		PartnerName:     partnerName,
		Region:          region,
	}
}

func extractVirtualInterface(vi *directconnect.VirtualInterface, region string) dcVirtualInterface {
	viId := ""
	if vi.VirtualInterfaceId != nil {
		viId = *vi.VirtualInterfaceId
	}
	name := ""
	if vi.VirtualInterfaceName != nil {
		name = *vi.VirtualInterfaceName
	}
	state := ""
	if vi.VirtualInterfaceState != nil {
		state = *vi.VirtualInterfaceState
	}
	viType := ""
	if vi.VirtualInterfaceType != nil {
		viType = *vi.VirtualInterfaceType
	}
	connId := ""
	if vi.ConnectionId != nil {
		connId = *vi.ConnectionId
	}
	vlan := ""
	if vi.Vlan != nil {
		vlan = fmt.Sprintf("%d", *vi.Vlan)
	}
	asn := ""
	if vi.Asn != nil {
		asn = fmt.Sprintf("%d", *vi.Asn)
	}
	amazonAddr := ""
	if vi.AmazonAddress != nil {
		amazonAddr = *vi.AmazonAddress
	}
	customerAddr := ""
	if vi.CustomerAddress != nil {
		customerAddr = *vi.CustomerAddress
	}
	return dcVirtualInterface{
		VirtualInterfaceId:    viId,
		VirtualInterfaceName:  name,
		VirtualInterfaceState: state,
		VirtualInterfaceType:  viType,
		ConnectionId:          connId,
		Vlan:                  vlan,
		Asn:                   asn,
		AmazonAddress:         amazonAddr,
		CustomerAddress:       customerAddr,
		Region:                region,
	}
}

func extractGateway(gw *directconnect.Gateway) dcGateway {
	gwId := ""
	if gw.DirectConnectGatewayId != nil {
		gwId = *gw.DirectConnectGatewayId
	}
	name := ""
	if gw.DirectConnectGatewayName != nil {
		name = *gw.DirectConnectGatewayName
	}
	state := ""
	if gw.DirectConnectGatewayState != nil {
		state = *gw.DirectConnectGatewayState
	}
	asn := ""
	if gw.AmazonSideAsn != nil {
		asn = fmt.Sprintf("%d", *gw.AmazonSideAsn)
	}
	owner := ""
	if gw.OwnerAccount != nil {
		owner = *gw.OwnerAccount
	}
	return dcGateway{
		DirectConnectGatewayId:    gwId,
		DirectConnectGatewayName:  name,
		DirectConnectGatewayState: state,
		AmazonSideAsn:             asn,
		OwnerAccount:              owner,
	}
}

var DirectConnectCalls = []types.AWSService{
	{
		Name: "directconnect:DescribeConnections",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			var allConnections []dcConnection
			var lastErr error

			for _, region := range types.Regions {
				svc := directconnect.New(sess, &aws.Config{Region: aws.String(region)})
				output, err := svc.DescribeConnectionsWithContext(ctx, &directconnect.DescribeConnectionsInput{})
				if err != nil {
					lastErr = err
					utils.HandleAWSError(false, "directconnect:DescribeConnections", err)
					continue
				}
				for _, conn := range output.Connections {
					if conn != nil {
						allConnections = append(allConnections, extractConnection(conn, region))
					}
				}
			}

			if len(allConnections) == 0 && lastErr != nil {
				return nil, lastErr
			}
			return allConnections, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "directconnect:DescribeConnections", err)
				return []types.ScanResult{
					{
						ServiceName: "DirectConnect",
						MethodName:  "directconnect:DescribeConnections",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			conns, ok := output.([]dcConnection)
			if !ok {
				utils.HandleAWSError(debug, "directconnect:DescribeConnections", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, conn := range conns {
				results = append(results, types.ScanResult{
					ServiceName:  "DirectConnect",
					MethodName:   "directconnect:DescribeConnections",
					ResourceType: "connection",
					ResourceName: conn.ConnectionName,
					Details: map[string]interface{}{
						"ConnectionId":    conn.ConnectionId,
						"ConnectionName":  conn.ConnectionName,
						"ConnectionState": conn.ConnectionState,
						"Bandwidth":       conn.Bandwidth,
						"Location":        conn.Location,
						"OwnerAccount":    conn.OwnerAccount,
						"PartnerName":     conn.PartnerName,
						"Region":          conn.Region,
					},
					Timestamp: time.Now(),
				})

				utils.PrintResult(debug, "", "directconnect:DescribeConnections",
					fmt.Sprintf("Direct Connect Connection: %s (State: %s, Bandwidth: %s, Location: %s, Region: %s)", utils.ColorizeItem(conn.ConnectionName), conn.ConnectionState, conn.Bandwidth, conn.Location, conn.Region), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
	{
		Name: "directconnect:DescribeVirtualInterfaces",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			var allVIs []dcVirtualInterface
			var lastErr error

			for _, region := range types.Regions {
				svc := directconnect.New(sess, &aws.Config{Region: aws.String(region)})
				output, err := svc.DescribeVirtualInterfacesWithContext(ctx, &directconnect.DescribeVirtualInterfacesInput{})
				if err != nil {
					lastErr = err
					utils.HandleAWSError(false, "directconnect:DescribeVirtualInterfaces", err)
					continue
				}
				for _, vi := range output.VirtualInterfaces {
					if vi != nil {
						allVIs = append(allVIs, extractVirtualInterface(vi, region))
					}
				}
			}

			if len(allVIs) == 0 && lastErr != nil {
				return nil, lastErr
			}
			return allVIs, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "directconnect:DescribeVirtualInterfaces", err)
				return []types.ScanResult{
					{
						ServiceName: "DirectConnect",
						MethodName:  "directconnect:DescribeVirtualInterfaces",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			vis, ok := output.([]dcVirtualInterface)
			if !ok {
				utils.HandleAWSError(debug, "directconnect:DescribeVirtualInterfaces", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, vi := range vis {
				results = append(results, types.ScanResult{
					ServiceName:  "DirectConnect",
					MethodName:   "directconnect:DescribeVirtualInterfaces",
					ResourceType: "virtual-interface",
					ResourceName: vi.VirtualInterfaceName,
					Details: map[string]interface{}{
						"VirtualInterfaceId":    vi.VirtualInterfaceId,
						"VirtualInterfaceName":  vi.VirtualInterfaceName,
						"VirtualInterfaceState": vi.VirtualInterfaceState,
						"VirtualInterfaceType":  vi.VirtualInterfaceType,
						"ConnectionId":          vi.ConnectionId,
						"Vlan":                  vi.Vlan,
						"Asn":                   vi.Asn,
						"AmazonAddress":         vi.AmazonAddress,
						"CustomerAddress":       vi.CustomerAddress,
						"Region":                vi.Region,
					},
					Timestamp: time.Now(),
				})

				utils.PrintResult(debug, "", "directconnect:DescribeVirtualInterfaces",
					fmt.Sprintf("Direct Connect Virtual Interface: %s (Type: %s, State: %s, VLAN: %s, Connection: %s, Region: %s)", utils.ColorizeItem(vi.VirtualInterfaceName), vi.VirtualInterfaceType, vi.VirtualInterfaceState, vi.Vlan, vi.ConnectionId, vi.Region), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
	{
		Name: "directconnect:DescribeDirectConnectGateways",
		Call: func(ctx context.Context, sess *session.Session) (interface{}, error) {
			var allGateways []dcGateway
			var lastErr error

			svc := directconnect.New(sess, &aws.Config{Region: aws.String(types.Regions[0])})
			var nextToken *string
			for {
				input := &directconnect.DescribeDirectConnectGatewaysInput{}
				if nextToken != nil {
					input.NextToken = nextToken
				}
				output, err := svc.DescribeDirectConnectGatewaysWithContext(ctx, input)
				if err != nil {
					lastErr = err
					utils.HandleAWSError(false, "directconnect:DescribeDirectConnectGateways", err)
					break
				}
				for _, gw := range output.DirectConnectGateways {
					if gw != nil {
						allGateways = append(allGateways, extractGateway(gw))
					}
				}
				if output.NextToken == nil {
					break
				}
				nextToken = output.NextToken
			}

			if len(allGateways) == 0 && lastErr != nil {
				return nil, lastErr
			}
			return allGateways, nil
		},
		Process: func(output interface{}, err error, debug bool) []types.ScanResult {
			var results []types.ScanResult

			if err != nil {
				utils.HandleAWSError(debug, "directconnect:DescribeDirectConnectGateways", err)
				return []types.ScanResult{
					{
						ServiceName: "DirectConnect",
						MethodName:  "directconnect:DescribeDirectConnectGateways",
						Error:       err,
						Timestamp:   time.Now(),
					},
				}
			}

			gws, ok := output.([]dcGateway)
			if !ok {
				utils.HandleAWSError(debug, "directconnect:DescribeDirectConnectGateways", fmt.Errorf("unexpected output type %T", output))
				return results
			}

			for _, gw := range gws {
				results = append(results, types.ScanResult{
					ServiceName:  "DirectConnect",
					MethodName:   "directconnect:DescribeDirectConnectGateways",
					ResourceType: "gateway",
					ResourceName: gw.DirectConnectGatewayName,
					Details: map[string]interface{}{
						"DirectConnectGatewayId":    gw.DirectConnectGatewayId,
						"DirectConnectGatewayName":  gw.DirectConnectGatewayName,
						"DirectConnectGatewayState": gw.DirectConnectGatewayState,
						"AmazonSideAsn":             gw.AmazonSideAsn,
						"OwnerAccount":              gw.OwnerAccount,
					},
					Timestamp: time.Now(),
				})

				utils.PrintResult(debug, "", "directconnect:DescribeDirectConnectGateways",
					fmt.Sprintf("Direct Connect Gateway: %s (State: %s, ASN: %s, Owner: %s)", utils.ColorizeItem(gw.DirectConnectGatewayName), gw.DirectConnectGatewayState, gw.AmazonSideAsn, gw.OwnerAccount), nil)
			}
			return results
		},
		ModuleName: types.DefaultModuleName,
	},
}
