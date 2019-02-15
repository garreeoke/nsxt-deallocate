package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
)

// Clean up orphaned nsx-t vip pool addresses
// #1 get all allocated IP addresses for the given ip pool id
// #2 For each load balancer, get the IP addresses from virtual servers
// #3 Get all IP addresses from the given T0 used for NAT
// #4 Match the list of vip pool addresses #1 to those found ... delete if they are not found

var t0 = flag.String("t0-id", "", "ID of t0 router")
var pool = flag.String("pool-id", "", "ID of ip pool to clean up")
var nsx = flag.String("nsx-ip", "", "IP address of nsx manager")
var user = flag.String("nsx-user", "admin", "Name of nsx users if other than admin")
var pass = flag.String("nsx-pass", "", "NSX password if not VMware1!")
var delete = flag.Bool("delete", false, "Execute the delete")

const (
	httpGet = "GET"
	httpPOST = "POST"
)

func main() {

	flag.Parse()

	client := http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
	// Get allocated ips
	allocatedIps, err := getAllocations(&client)
	if err != nil {
		fmt.Println(err)
	}

	// Get load balancer used ips
	lbIps, err := getLBIps(&client)
	if err != nil {
		fmt.Println(err)
	}

	// Get Nat rule ips
	natIps, err := getNatIps(&client)
	if err != nil {
		fmt.Println(err)
	}

	usedIps := append(lbIps,natIps...)
	var deleteIps []string
	for _,allocatedIp := range allocatedIps {
		used := 0
		for _, usedIp := range usedIps {
			if usedIp == allocatedIp {
				used++
			}
		}
		if used == 0 {
			deleteIps = append(deleteIps, allocatedIp)
		}
	}

	for _,deleteIp := range deleteIps {
		fmt.Println("OrphanedIP: ", deleteIp)
	}

	if len(deleteIps) > 0 {
		if *delete {
			fmt.Println("Deleting Ips")
			unallocateIps(&client, deleteIps)
		}
	} else {
		fmt.Println("No orphaned IPs to delete")
	}

}

func unallocateIps (c *http.Client, ips []string) error {

	for _,ip := range ips {
		fmt.Println("DeletingIP: ", ip)
		url := "https://" + *nsx + "/api/v1/pools/ip-pools/" + *pool + "?action=RELEASE"
		m := map[string]string{
			"allocation_id" : ip,
		}
		payload, err := json.Marshal(m)
		if err != nil {
			return err
		}
		_, err = httpCall(c, url, httpPOST, payload)
		if err != nil {
			return err
		}
		fmt.Println("Successfully deleted: ", ip)

	}
	return nil
}

func getNatIps (c *http.Client) ([]string, error) {

	url := "https://" + *nsx + "/api/v1/logical-routers/" + *t0 + "/nat/rules"
	result, err := httpCall(c, url, httpGet, nil)
	if err != nil {
		return nil, err
	}
	natIps := make([]string, len(result.Results))
	for i,res := range result.Results {
		natIps[i] = res.TranIP
	}

	return natIps, nil
}

// Get LoadBalancer ids
func getLBIps(c *http.Client) ([]string, error) {

	url := "https://" + *nsx + "/api/v1/loadbalancer/services"
	result, err := httpCall(c, url, httpGet, nil)
	if err != nil {
		return nil, err
	}

	var lbIps []string
	for _, res := range result.Results {
		// Get virtual server ids
		for _, vsi := range res.VirtualServerIds {
			// Get IP address and add it
			url2 := "https://" + *nsx + "/api/v1/loadbalancer/virtual-servers"
			result2, err := httpCall(c, url2, httpGet, nil)
			if err != nil {
				return nil, err
			}
			for _,virtualServer := range result2.Results {
				if virtualServer.ID == vsi {
					check := 0
					for _,l := range lbIps {
						if l == virtualServer.IP {
							check++
						}
					}
					if check == 0 {
						lbIps = append(lbIps,virtualServer.IP)
					}
				}
			}
		}
	}

	return lbIps, nil
}

func getAllocations(c *http.Client) ([]string, error) {

	url := "https://" + *nsx + "/api/v1/pools/ip-pools/" + *pool + "/allocations"
	result, err := httpCall(c, url, httpGet, nil)
	if err != nil {
		return nil, err
	}

	// Generate the list
	ips := make([]string, len(result.Results))
	for i, res := range result.Results {
		ips[i] = res.AllocationID
	}

	return ips, nil
}

type Data struct {
	Count   int      `json:"result_count,omitempty"`
	Results []Result `json:"results,omitempty"`
}

type Result struct {
	ID               string   `json:"id,omitempty"`
	AllocationID     string   `json:"allocation_id,omitempty"`
	VirtualServerIds []string `json:"virtual_server_ids,omitempty"`
	IP               string   `json:"ip_address,omitempty"`
	TranIP           string   `json:"translated_network,omitempty"`
}

func httpCall(c *http.Client, url,method string, data []byte) (*Data, error) {
	req, err := http.NewRequest(method, url, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.SetBasicAuth(*user, *pass)
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}

	if method == httpGet {
		r, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		resp.Body.Close()

		var result Data
		err = json.Unmarshal(r, &result)
		if err != nil {
			return nil, err
		}
		return &result, nil
	} else {
		return nil, nil
	}
}
