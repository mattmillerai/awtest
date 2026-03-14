package directconnect

import (
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/directconnect"
)

func TestDescribeConnectionsProcess(t *testing.T) {
	process := DirectConnectCalls[0].Process

	tests := []struct {
		name             string
		output           interface{}
		err              error
		wantLen          int
		wantError        bool
		wantResourceName string
		wantConnId       string
		wantConnName     string
		wantState        string
		wantBandwidth    string
		wantLocation     string
		wantOwner        string
		wantPartner      string
		wantRegion       string
	}{
		{
			name: "valid connections with full details",
			output: []dcConnection{
				{
					ConnectionId:    "dxcon-abc12345",
					ConnectionName:  "my-dx-connection",
					ConnectionState: "available",
					Bandwidth:       "1Gbps",
					Location:        "EqDC2",
					OwnerAccount:    "111111111111",
					PartnerName:     "Equinix",
					Region:          "us-east-1",
				},
				{
					ConnectionId:    "dxcon-def67890",
					ConnectionName:  "backup-connection",
					ConnectionState: "down",
					Bandwidth:       "10Gbps",
					Location:        "CoreSite",
					OwnerAccount:    "222222222222",
					PartnerName:     "CoreSite",
					Region:          "us-west-2",
				},
			},
			wantLen:          2,
			wantResourceName: "my-dx-connection",
			wantConnId:       "dxcon-abc12345",
			wantConnName:     "my-dx-connection",
			wantState:        "available",
			wantBandwidth:    "1Gbps",
			wantLocation:     "EqDC2",
			wantOwner:        "111111111111",
			wantPartner:      "Equinix",
			wantRegion:       "us-east-1",
		},
		{
			name:    "empty results",
			output:  []dcConnection{},
			wantLen: 0,
		},
		{
			name:      "access denied error",
			output:    nil,
			err:       fmt.Errorf("AccessDeniedException: User is not authorized"),
			wantLen:   1,
			wantError: true,
		},
		{
			name: "nil-safe fields (empty strings)",
			output: []dcConnection{
				{
					ConnectionId:    "",
					ConnectionName:  "",
					ConnectionState: "",
					Bandwidth:       "",
					Location:        "",
					OwnerAccount:    "",
					PartnerName:     "",
					Region:          "",
				},
			},
			wantLen:          1,
			wantResourceName: "",
			wantConnId:       "",
			wantConnName:     "",
			wantState:        "",
			wantBandwidth:    "",
			wantLocation:     "",
			wantOwner:        "",
			wantPartner:      "",
			wantRegion:       "",
		},
		{
			name:    "type assertion failure",
			output:  "invalid type",
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := process(tt.output, tt.err, false)

			if len(results) != tt.wantLen {
				t.Fatalf("expected %d results, got %d", tt.wantLen, len(results))
			}

			if tt.wantError {
				if results[0].Error == nil {
					t.Error("expected error in result, got nil")
				}
				if results[0].ServiceName != "DirectConnect" {
					t.Errorf("expected ServiceName 'DirectConnect', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "directconnect:DescribeConnections" {
					t.Errorf("expected MethodName 'directconnect:DescribeConnections', got '%s'", results[0].MethodName)
				}
				return
			}

			if tt.wantLen > 0 {
				if results[0].ServiceName != "DirectConnect" {
					t.Errorf("expected ServiceName 'DirectConnect', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "directconnect:DescribeConnections" {
					t.Errorf("expected MethodName 'directconnect:DescribeConnections', got '%s'", results[0].MethodName)
				}
				if results[0].ResourceType != "connection" {
					t.Errorf("expected ResourceType 'connection', got '%s'", results[0].ResourceType)
				}
				if results[0].ResourceName != tt.wantResourceName {
					t.Errorf("expected ResourceName '%s', got '%s'", tt.wantResourceName, results[0].ResourceName)
				}
				if connId, ok := results[0].Details["ConnectionId"].(string); ok {
					if connId != tt.wantConnId {
						t.Errorf("expected ConnectionId '%s', got '%s'", tt.wantConnId, connId)
					}
				} else if tt.wantConnId != "" {
					t.Errorf("expected ConnectionId in Details, got none")
				}
				if connName, ok := results[0].Details["ConnectionName"].(string); ok {
					if connName != tt.wantConnName {
						t.Errorf("expected ConnectionName '%s', got '%s'", tt.wantConnName, connName)
					}
				} else if tt.wantConnName != "" {
					t.Errorf("expected ConnectionName in Details, got none")
				}
				if state, ok := results[0].Details["ConnectionState"].(string); ok {
					if state != tt.wantState {
						t.Errorf("expected ConnectionState '%s', got '%s'", tt.wantState, state)
					}
				} else if tt.wantState != "" {
					t.Errorf("expected ConnectionState in Details, got none")
				}
				if bw, ok := results[0].Details["Bandwidth"].(string); ok {
					if bw != tt.wantBandwidth {
						t.Errorf("expected Bandwidth '%s', got '%s'", tt.wantBandwidth, bw)
					}
				} else if tt.wantBandwidth != "" {
					t.Errorf("expected Bandwidth in Details, got none")
				}
				if loc, ok := results[0].Details["Location"].(string); ok {
					if loc != tt.wantLocation {
						t.Errorf("expected Location '%s', got '%s'", tt.wantLocation, loc)
					}
				} else if tt.wantLocation != "" {
					t.Errorf("expected Location in Details, got none")
				}
				if owner, ok := results[0].Details["OwnerAccount"].(string); ok {
					if owner != tt.wantOwner {
						t.Errorf("expected OwnerAccount '%s', got '%s'", tt.wantOwner, owner)
					}
				} else if tt.wantOwner != "" {
					t.Errorf("expected OwnerAccount in Details, got none")
				}
				if partner, ok := results[0].Details["PartnerName"].(string); ok {
					if partner != tt.wantPartner {
						t.Errorf("expected PartnerName '%s', got '%s'", tt.wantPartner, partner)
					}
				} else if tt.wantPartner != "" {
					t.Errorf("expected PartnerName in Details, got none")
				}
				if region, ok := results[0].Details["Region"].(string); ok {
					if region != tt.wantRegion {
						t.Errorf("expected Region '%s', got '%s'", tt.wantRegion, region)
					}
				} else if tt.wantRegion != "" {
					t.Errorf("expected Region in Details, got none")
				}
			}
		})
	}
}

func TestDescribeVirtualInterfacesProcess(t *testing.T) {
	process := DirectConnectCalls[1].Process

	tests := []struct {
		name             string
		output           interface{}
		err              error
		wantLen          int
		wantError        bool
		wantResourceName string
		wantVIId         string
		wantVIName       string
		wantState        string
		wantType         string
		wantConnId       string
		wantVlan         string
		wantAsn          string
		wantAmazonAddr   string
		wantCustAddr     string
		wantRegion       string
	}{
		{
			name: "valid virtual interfaces with full details",
			output: []dcVirtualInterface{
				{
					VirtualInterfaceId:    "dxvif-abc12345",
					VirtualInterfaceName:  "my-private-vif",
					VirtualInterfaceState: "available",
					VirtualInterfaceType:  "private",
					ConnectionId:          "dxcon-abc12345",
					Vlan:                  "100",
					Asn:                   "65000",
					AmazonAddress:         "175.45.176.1/30",
					CustomerAddress:       "175.45.176.2/30",
					Region:                "us-east-1",
				},
				{
					VirtualInterfaceId:    "dxvif-def67890",
					VirtualInterfaceName:  "my-public-vif",
					VirtualInterfaceState: "down",
					VirtualInterfaceType:  "public",
					ConnectionId:          "dxcon-def67890",
					Vlan:                  "200",
					Asn:                   "65001",
					AmazonAddress:         "10.0.0.1/30",
					CustomerAddress:       "10.0.0.2/30",
					Region:                "us-west-2",
				},
			},
			wantLen:          2,
			wantResourceName: "my-private-vif",
			wantVIId:         "dxvif-abc12345",
			wantVIName:       "my-private-vif",
			wantState:        "available",
			wantType:         "private",
			wantConnId:       "dxcon-abc12345",
			wantVlan:         "100",
			wantAsn:          "65000",
			wantAmazonAddr:   "175.45.176.1/30",
			wantCustAddr:     "175.45.176.2/30",
			wantRegion:       "us-east-1",
		},
		{
			name:    "empty results",
			output:  []dcVirtualInterface{},
			wantLen: 0,
		},
		{
			name:      "access denied error",
			output:    nil,
			err:       fmt.Errorf("AccessDeniedException: User is not authorized"),
			wantLen:   1,
			wantError: true,
		},
		{
			name: "nil-safe fields (empty strings)",
			output: []dcVirtualInterface{
				{
					VirtualInterfaceId:    "",
					VirtualInterfaceName:  "",
					VirtualInterfaceState: "",
					VirtualInterfaceType:  "",
					ConnectionId:          "",
					Vlan:                  "",
					Asn:                   "",
					AmazonAddress:         "",
					CustomerAddress:       "",
					Region:                "",
				},
			},
			wantLen:          1,
			wantResourceName: "",
			wantVIId:         "",
			wantVIName:       "",
			wantState:        "",
			wantType:         "",
			wantConnId:       "",
			wantVlan:         "",
			wantAsn:          "",
			wantAmazonAddr:   "",
			wantCustAddr:     "",
			wantRegion:       "",
		},
		{
			name:    "type assertion failure",
			output:  "invalid type",
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := process(tt.output, tt.err, false)

			if len(results) != tt.wantLen {
				t.Fatalf("expected %d results, got %d", tt.wantLen, len(results))
			}

			if tt.wantError {
				if results[0].Error == nil {
					t.Error("expected error in result, got nil")
				}
				if results[0].ServiceName != "DirectConnect" {
					t.Errorf("expected ServiceName 'DirectConnect', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "directconnect:DescribeVirtualInterfaces" {
					t.Errorf("expected MethodName 'directconnect:DescribeVirtualInterfaces', got '%s'", results[0].MethodName)
				}
				return
			}

			if tt.wantLen > 0 {
				if results[0].ServiceName != "DirectConnect" {
					t.Errorf("expected ServiceName 'DirectConnect', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "directconnect:DescribeVirtualInterfaces" {
					t.Errorf("expected MethodName 'directconnect:DescribeVirtualInterfaces', got '%s'", results[0].MethodName)
				}
				if results[0].ResourceType != "virtual-interface" {
					t.Errorf("expected ResourceType 'virtual-interface', got '%s'", results[0].ResourceType)
				}
				if results[0].ResourceName != tt.wantResourceName {
					t.Errorf("expected ResourceName '%s', got '%s'", tt.wantResourceName, results[0].ResourceName)
				}
				if viId, ok := results[0].Details["VirtualInterfaceId"].(string); ok {
					if viId != tt.wantVIId {
						t.Errorf("expected VirtualInterfaceId '%s', got '%s'", tt.wantVIId, viId)
					}
				} else if tt.wantVIId != "" {
					t.Errorf("expected VirtualInterfaceId in Details, got none")
				}
				if viName, ok := results[0].Details["VirtualInterfaceName"].(string); ok {
					if viName != tt.wantVIName {
						t.Errorf("expected VirtualInterfaceName '%s', got '%s'", tt.wantVIName, viName)
					}
				} else if tt.wantVIName != "" {
					t.Errorf("expected VirtualInterfaceName in Details, got none")
				}
				if state, ok := results[0].Details["VirtualInterfaceState"].(string); ok {
					if state != tt.wantState {
						t.Errorf("expected VirtualInterfaceState '%s', got '%s'", tt.wantState, state)
					}
				} else if tt.wantState != "" {
					t.Errorf("expected VirtualInterfaceState in Details, got none")
				}
				if viType, ok := results[0].Details["VirtualInterfaceType"].(string); ok {
					if viType != tt.wantType {
						t.Errorf("expected VirtualInterfaceType '%s', got '%s'", tt.wantType, viType)
					}
				} else if tt.wantType != "" {
					t.Errorf("expected VirtualInterfaceType in Details, got none")
				}
				if connId, ok := results[0].Details["ConnectionId"].(string); ok {
					if connId != tt.wantConnId {
						t.Errorf("expected ConnectionId '%s', got '%s'", tt.wantConnId, connId)
					}
				} else if tt.wantConnId != "" {
					t.Errorf("expected ConnectionId in Details, got none")
				}
				if vlan, ok := results[0].Details["Vlan"].(string); ok {
					if vlan != tt.wantVlan {
						t.Errorf("expected Vlan '%s', got '%s'", tt.wantVlan, vlan)
					}
				} else if tt.wantVlan != "" {
					t.Errorf("expected Vlan in Details, got none")
				}
				if asn, ok := results[0].Details["Asn"].(string); ok {
					if asn != tt.wantAsn {
						t.Errorf("expected Asn '%s', got '%s'", tt.wantAsn, asn)
					}
				} else if tt.wantAsn != "" {
					t.Errorf("expected Asn in Details, got none")
				}
				if addr, ok := results[0].Details["AmazonAddress"].(string); ok {
					if addr != tt.wantAmazonAddr {
						t.Errorf("expected AmazonAddress '%s', got '%s'", tt.wantAmazonAddr, addr)
					}
				} else if tt.wantAmazonAddr != "" {
					t.Errorf("expected AmazonAddress in Details, got none")
				}
				if addr, ok := results[0].Details["CustomerAddress"].(string); ok {
					if addr != tt.wantCustAddr {
						t.Errorf("expected CustomerAddress '%s', got '%s'", tt.wantCustAddr, addr)
					}
				} else if tt.wantCustAddr != "" {
					t.Errorf("expected CustomerAddress in Details, got none")
				}
				if region, ok := results[0].Details["Region"].(string); ok {
					if region != tt.wantRegion {
						t.Errorf("expected Region '%s', got '%s'", tt.wantRegion, region)
					}
				} else if tt.wantRegion != "" {
					t.Errorf("expected Region in Details, got none")
				}
			}
		})
	}
}

func TestDescribeDirectConnectGatewaysProcess(t *testing.T) {
	process := DirectConnectCalls[2].Process

	tests := []struct {
		name             string
		output           interface{}
		err              error
		wantLen          int
		wantError        bool
		wantResourceName string
		wantGwId         string
		wantGwName       string
		wantState        string
		wantAsn          string
		wantOwner        string
	}{
		{
			name: "valid gateways with full details",
			output: []dcGateway{
				{
					DirectConnectGatewayId:    "dgw-abc12345",
					DirectConnectGatewayName:  "my-dx-gateway",
					DirectConnectGatewayState: "available",
					AmazonSideAsn:             "64512",
					OwnerAccount:              "111111111111",
				},
				{
					DirectConnectGatewayId:    "dgw-def67890",
					DirectConnectGatewayName:  "backup-gateway",
					DirectConnectGatewayState: "pending",
					AmazonSideAsn:             "64513",
					OwnerAccount:              "222222222222",
				},
			},
			wantLen:          2,
			wantResourceName: "my-dx-gateway",
			wantGwId:         "dgw-abc12345",
			wantGwName:       "my-dx-gateway",
			wantState:        "available",
			wantAsn:          "64512",
			wantOwner:        "111111111111",
		},
		{
			name:    "empty results",
			output:  []dcGateway{},
			wantLen: 0,
		},
		{
			name:      "access denied error",
			output:    nil,
			err:       fmt.Errorf("AccessDeniedException: User is not authorized"),
			wantLen:   1,
			wantError: true,
		},
		{
			name: "nil-safe fields (empty strings)",
			output: []dcGateway{
				{
					DirectConnectGatewayId:    "",
					DirectConnectGatewayName:  "",
					DirectConnectGatewayState: "",
					AmazonSideAsn:             "",
					OwnerAccount:              "",
				},
			},
			wantLen:          1,
			wantResourceName: "",
			wantGwId:         "",
			wantGwName:       "",
			wantState:        "",
			wantAsn:          "",
			wantOwner:        "",
		},
		{
			name:    "type assertion failure",
			output:  "invalid type",
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := process(tt.output, tt.err, false)

			if len(results) != tt.wantLen {
				t.Fatalf("expected %d results, got %d", tt.wantLen, len(results))
			}

			if tt.wantError {
				if results[0].Error == nil {
					t.Error("expected error in result, got nil")
				}
				if results[0].ServiceName != "DirectConnect" {
					t.Errorf("expected ServiceName 'DirectConnect', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "directconnect:DescribeDirectConnectGateways" {
					t.Errorf("expected MethodName 'directconnect:DescribeDirectConnectGateways', got '%s'", results[0].MethodName)
				}
				return
			}

			if tt.wantLen > 0 {
				if results[0].ServiceName != "DirectConnect" {
					t.Errorf("expected ServiceName 'DirectConnect', got '%s'", results[0].ServiceName)
				}
				if results[0].MethodName != "directconnect:DescribeDirectConnectGateways" {
					t.Errorf("expected MethodName 'directconnect:DescribeDirectConnectGateways', got '%s'", results[0].MethodName)
				}
				if results[0].ResourceType != "gateway" {
					t.Errorf("expected ResourceType 'gateway', got '%s'", results[0].ResourceType)
				}
				if results[0].ResourceName != tt.wantResourceName {
					t.Errorf("expected ResourceName '%s', got '%s'", tt.wantResourceName, results[0].ResourceName)
				}
				if gwId, ok := results[0].Details["DirectConnectGatewayId"].(string); ok {
					if gwId != tt.wantGwId {
						t.Errorf("expected DirectConnectGatewayId '%s', got '%s'", tt.wantGwId, gwId)
					}
				} else if tt.wantGwId != "" {
					t.Errorf("expected DirectConnectGatewayId in Details, got none")
				}
				if gwName, ok := results[0].Details["DirectConnectGatewayName"].(string); ok {
					if gwName != tt.wantGwName {
						t.Errorf("expected DirectConnectGatewayName '%s', got '%s'", tt.wantGwName, gwName)
					}
				} else if tt.wantGwName != "" {
					t.Errorf("expected DirectConnectGatewayName in Details, got none")
				}
				if state, ok := results[0].Details["DirectConnectGatewayState"].(string); ok {
					if state != tt.wantState {
						t.Errorf("expected DirectConnectGatewayState '%s', got '%s'", tt.wantState, state)
					}
				} else if tt.wantState != "" {
					t.Errorf("expected DirectConnectGatewayState in Details, got none")
				}
				if asn, ok := results[0].Details["AmazonSideAsn"].(string); ok {
					if asn != tt.wantAsn {
						t.Errorf("expected AmazonSideAsn '%s', got '%s'", tt.wantAsn, asn)
					}
				} else if tt.wantAsn != "" {
					t.Errorf("expected AmazonSideAsn in Details, got none")
				}
				if owner, ok := results[0].Details["OwnerAccount"].(string); ok {
					if owner != tt.wantOwner {
						t.Errorf("expected OwnerAccount '%s', got '%s'", tt.wantOwner, owner)
					}
				} else if tt.wantOwner != "" {
					t.Errorf("expected OwnerAccount in Details, got none")
				}
			}
		})
	}
}

func TestExtractConnection(t *testing.T) {
	tests := []struct {
		name        string
		input       *directconnect.Connection
		region      string
		wantConnId  string
		wantName    string
		wantState   string
		wantBW      string
		wantLoc     string
		wantOwner   string
		wantPartner string
		wantRegion  string
	}{
		{
			name: "all fields populated",
			input: &directconnect.Connection{
				ConnectionId:    aws.String("dxcon-abc12345"),
				ConnectionName:  aws.String("my-connection"),
				ConnectionState: aws.String("available"),
				Bandwidth:       aws.String("1Gbps"),
				Location:        aws.String("EqDC2"),
				OwnerAccount:    aws.String("111111111111"),
				PartnerName:     aws.String("Equinix"),
			},
			region:      "us-east-1",
			wantConnId:  "dxcon-abc12345",
			wantName:    "my-connection",
			wantState:   "available",
			wantBW:      "1Gbps",
			wantLoc:     "EqDC2",
			wantOwner:   "111111111111",
			wantPartner: "Equinix",
			wantRegion:  "us-east-1",
		},
		{
			name:        "all fields nil",
			input:       &directconnect.Connection{},
			region:      "eu-west-1",
			wantConnId:  "",
			wantName:    "",
			wantState:   "",
			wantBW:      "",
			wantLoc:     "",
			wantOwner:   "",
			wantPartner: "",
			wantRegion:  "eu-west-1",
		},
		{
			name: "partial fields populated",
			input: &directconnect.Connection{
				ConnectionId:   aws.String("dxcon-partial"),
				ConnectionName: aws.String("partial-conn"),
			},
			region:      "ap-southeast-1",
			wantConnId:  "dxcon-partial",
			wantName:    "partial-conn",
			wantState:   "",
			wantBW:      "",
			wantLoc:     "",
			wantOwner:   "",
			wantPartner: "",
			wantRegion:  "ap-southeast-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractConnection(tt.input, tt.region)
			if result.ConnectionId != tt.wantConnId {
				t.Errorf("ConnectionId: got %q, want %q", result.ConnectionId, tt.wantConnId)
			}
			if result.ConnectionName != tt.wantName {
				t.Errorf("ConnectionName: got %q, want %q", result.ConnectionName, tt.wantName)
			}
			if result.ConnectionState != tt.wantState {
				t.Errorf("ConnectionState: got %q, want %q", result.ConnectionState, tt.wantState)
			}
			if result.Bandwidth != tt.wantBW {
				t.Errorf("Bandwidth: got %q, want %q", result.Bandwidth, tt.wantBW)
			}
			if result.Location != tt.wantLoc {
				t.Errorf("Location: got %q, want %q", result.Location, tt.wantLoc)
			}
			if result.OwnerAccount != tt.wantOwner {
				t.Errorf("OwnerAccount: got %q, want %q", result.OwnerAccount, tt.wantOwner)
			}
			if result.PartnerName != tt.wantPartner {
				t.Errorf("PartnerName: got %q, want %q", result.PartnerName, tt.wantPartner)
			}
			if result.Region != tt.wantRegion {
				t.Errorf("Region: got %q, want %q", result.Region, tt.wantRegion)
			}
		})
	}
}

func TestExtractVirtualInterface(t *testing.T) {
	tests := []struct {
		name       string
		input      *directconnect.VirtualInterface
		region     string
		wantVIId   string
		wantName   string
		wantState  string
		wantType   string
		wantConnId string
		wantVlan   string
		wantAsn    string
		wantAmzn   string
		wantCust   string
		wantRegion string
	}{
		{
			name: "all fields populated",
			input: &directconnect.VirtualInterface{
				VirtualInterfaceId:    aws.String("dxvif-abc12345"),
				VirtualInterfaceName:  aws.String("my-vif"),
				VirtualInterfaceState: aws.String("available"),
				VirtualInterfaceType:  aws.String("private"),
				ConnectionId:          aws.String("dxcon-abc12345"),
				Vlan:                  aws.Int64(100),
				Asn:                   aws.Int64(65000),
				AmazonAddress:         aws.String("175.45.176.1/30"),
				CustomerAddress:       aws.String("175.45.176.2/30"),
			},
			region:     "us-east-1",
			wantVIId:   "dxvif-abc12345",
			wantName:   "my-vif",
			wantState:  "available",
			wantType:   "private",
			wantConnId: "dxcon-abc12345",
			wantVlan:   "100",
			wantAsn:    "65000",
			wantAmzn:   "175.45.176.1/30",
			wantCust:   "175.45.176.2/30",
			wantRegion: "us-east-1",
		},
		{
			name:       "all fields nil",
			input:      &directconnect.VirtualInterface{},
			region:     "eu-west-1",
			wantVIId:   "",
			wantName:   "",
			wantState:  "",
			wantType:   "",
			wantConnId: "",
			wantVlan:   "",
			wantAsn:    "",
			wantAmzn:   "",
			wantCust:   "",
			wantRegion: "eu-west-1",
		},
		{
			name: "zero-value integers",
			input: &directconnect.VirtualInterface{
				VirtualInterfaceId:   aws.String("dxvif-zero"),
				VirtualInterfaceName: aws.String("zero-vif"),
				Vlan:                 aws.Int64(0),
				Asn:                  aws.Int64(0),
			},
			region:     "us-west-2",
			wantVIId:   "dxvif-zero",
			wantName:   "zero-vif",
			wantVlan:   "0",
			wantAsn:    "0",
			wantRegion: "us-west-2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractVirtualInterface(tt.input, tt.region)
			if result.VirtualInterfaceId != tt.wantVIId {
				t.Errorf("VirtualInterfaceId: got %q, want %q", result.VirtualInterfaceId, tt.wantVIId)
			}
			if result.VirtualInterfaceName != tt.wantName {
				t.Errorf("VirtualInterfaceName: got %q, want %q", result.VirtualInterfaceName, tt.wantName)
			}
			if result.VirtualInterfaceState != tt.wantState {
				t.Errorf("VirtualInterfaceState: got %q, want %q", result.VirtualInterfaceState, tt.wantState)
			}
			if result.VirtualInterfaceType != tt.wantType {
				t.Errorf("VirtualInterfaceType: got %q, want %q", result.VirtualInterfaceType, tt.wantType)
			}
			if result.ConnectionId != tt.wantConnId {
				t.Errorf("ConnectionId: got %q, want %q", result.ConnectionId, tt.wantConnId)
			}
			if result.Vlan != tt.wantVlan {
				t.Errorf("Vlan: got %q, want %q", result.Vlan, tt.wantVlan)
			}
			if result.Asn != tt.wantAsn {
				t.Errorf("Asn: got %q, want %q", result.Asn, tt.wantAsn)
			}
			if result.AmazonAddress != tt.wantAmzn {
				t.Errorf("AmazonAddress: got %q, want %q", result.AmazonAddress, tt.wantAmzn)
			}
			if result.CustomerAddress != tt.wantCust {
				t.Errorf("CustomerAddress: got %q, want %q", result.CustomerAddress, tt.wantCust)
			}
			if result.Region != tt.wantRegion {
				t.Errorf("Region: got %q, want %q", result.Region, tt.wantRegion)
			}
		})
	}
}

func TestExtractGateway(t *testing.T) {
	tests := []struct {
		name      string
		input     *directconnect.Gateway
		wantGwId  string
		wantName  string
		wantState string
		wantAsn   string
		wantOwner string
	}{
		{
			name: "all fields populated",
			input: &directconnect.Gateway{
				DirectConnectGatewayId:    aws.String("dgw-abc12345"),
				DirectConnectGatewayName:  aws.String("my-gateway"),
				DirectConnectGatewayState: aws.String("available"),
				AmazonSideAsn:             aws.Int64(64512),
				OwnerAccount:              aws.String("111111111111"),
			},
			wantGwId:  "dgw-abc12345",
			wantName:  "my-gateway",
			wantState: "available",
			wantAsn:   "64512",
			wantOwner: "111111111111",
		},
		{
			name:      "all fields nil",
			input:     &directconnect.Gateway{},
			wantGwId:  "",
			wantName:  "",
			wantState: "",
			wantAsn:   "",
			wantOwner: "",
		},
		{
			name: "zero-value ASN",
			input: &directconnect.Gateway{
				DirectConnectGatewayId:   aws.String("dgw-zero"),
				DirectConnectGatewayName: aws.String("zero-gw"),
				AmazonSideAsn:            aws.Int64(0),
			},
			wantGwId:  "dgw-zero",
			wantName:  "zero-gw",
			wantState: "",
			wantAsn:   "0",
			wantOwner: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractGateway(tt.input)
			if result.DirectConnectGatewayId != tt.wantGwId {
				t.Errorf("DirectConnectGatewayId: got %q, want %q", result.DirectConnectGatewayId, tt.wantGwId)
			}
			if result.DirectConnectGatewayName != tt.wantName {
				t.Errorf("DirectConnectGatewayName: got %q, want %q", result.DirectConnectGatewayName, tt.wantName)
			}
			if result.DirectConnectGatewayState != tt.wantState {
				t.Errorf("DirectConnectGatewayState: got %q, want %q", result.DirectConnectGatewayState, tt.wantState)
			}
			if result.AmazonSideAsn != tt.wantAsn {
				t.Errorf("AmazonSideAsn: got %q, want %q", result.AmazonSideAsn, tt.wantAsn)
			}
			if result.OwnerAccount != tt.wantOwner {
				t.Errorf("OwnerAccount: got %q, want %q", result.OwnerAccount, tt.wantOwner)
			}
		})
	}
}
