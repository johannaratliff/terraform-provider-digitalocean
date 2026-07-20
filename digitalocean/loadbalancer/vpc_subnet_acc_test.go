package loadbalancer_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/digitalocean/godo"
	"github.com/digitalocean/terraform-provider-digitalocean/digitalocean/acceptance"
	"github.com/digitalocean/terraform-provider-digitalocean/digitalocean/config"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

type vpcSubnetCreateRequest struct {
	Name    string `json:"name"`
	IPRange string `json:"ip_range"`
}

type vpcSubnetResponse struct {
	VPCSubnet struct {
		ID string `json:"id"`
	} `json:"vpc_subnet"`
}

func accTestGodoClient(t *testing.T) *godo.Client {
	t.Helper()
	acceptance.TestAccPreCheck(t)
	return acceptance.TestAccProvider.Meta().(*config.CombinedConfig).GodoClient()
}

func createAccTestVPCSubnet(ctx context.Context, client *godo.Client, vpcUUID, name, ipRange string) (string, error) {
	path := fmt.Sprintf("/v2/vpcs/%s/subnets", vpcUUID)
	req, err := client.NewRequest(ctx, http.MethodPost, path, &vpcSubnetCreateRequest{
		Name:    name,
		IPRange: ipRange,
	})
	if err != nil {
		return "", err
	}

	root := new(vpcSubnetResponse)
	if _, err := client.Do(ctx, req, root); err != nil {
		return "", err
	}

	if root.VPCSubnet.ID == "" {
		return "", fmt.Errorf("create vpc subnet returned empty id")
	}

	return root.VPCSubnet.ID, nil
}

func deleteAccTestVPCSubnet(ctx context.Context, client *godo.Client, vpcUUID, subnetUUID string) error {
	path := fmt.Sprintf("/v2/vpcs/%s/subnets/%s", vpcUUID, subnetUUID)
	req, err := client.NewRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}

	_, err = client.Do(ctx, req, nil)
	return err
}

func testAccCreateVPCSubnet(vpcResourceName string, subnetUUID *string, t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[vpcResourceName]
		if !ok {
			return fmt.Errorf("resource %s not found", vpcResourceName)
		}

		client := accTestGodoClient(t)
		subnetName := acceptance.RandomTestName("subnet")
		id, err := createAccTestVPCSubnet(context.Background(), client, rs.Primary.ID, subnetName, "10.10.1.0/24")
		if err != nil {
			return fmt.Errorf("failed to create vpc subnet: %w", err)
		}

		*subnetUUID = id
		vpcUUID := rs.Primary.ID

		t.Cleanup(func() {
			if err := deleteAccTestVPCSubnet(context.Background(), client, vpcUUID, id); err != nil {
				t.Logf("failed to delete acc test vpc subnet %s: %v", id, err)
			}
		})

		return nil
	}
}

func testAccCheckLoadBalancerSubnetUUID(lbResourceName, expectedSubnetUUID string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[lbResourceName]
		if !ok {
			return fmt.Errorf("resource %s not found", lbResourceName)
		}

		got := rs.Primary.Attributes["subnet_uuid"]
		if got != expectedSubnetUUID {
			return fmt.Errorf("subnet_uuid %q, want %q", got, expectedSubnetUUID)
		}

		return nil
	}
}

func testAccCheckSubnetUUIDDiffersFromVPCUUID(resourceName, vpcResourceName, subnetAttr string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource %s not found", resourceName)
		}

		vpc, ok := s.RootModule().Resources[vpcResourceName]
		if !ok {
			return fmt.Errorf("resource %s not found", vpcResourceName)
		}

		subnetUUID := rs.Primary.Attributes[subnetAttr]
		if subnetUUID == vpc.Primary.ID {
			return fmt.Errorf("%s must differ from vpc uuid %q", subnetAttr, vpc.Primary.ID)
		}

		return nil
	}
}
