package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

// Trace flow
var nsx = flag.String("nsx-ip", "", "IP address of nsx manager")
var user = flag.String("nsx-user", "admin", "Name of nsx users if other than admin")
var project = flag.String("namespace", "", "Kubernetes namespace")
var srcPod = flag.String("src-pod", "", "Pod name of the source")
var srcPort = flag.String("src-port", "8080", "Port for the source")
var dstPod = flag.String("dst-pod", "", "Pod name of the destination")
var dstPort = flag.String("dst-port", "", "Port for the destination")

//var proto = flag.String("proto", "tcp", "Protocol for the trace")
var payload = flag.String("payload", "", "Name of nsx users if other than admin")
var pass = flag.String("nsx-pass", "", "NSX password")
var dstProject = flag.String("dst-namespace", "", "Kubernetes namespace")

const (
	httpGet  = "GET"
	httpPost = "POST"
)

type Args struct {
	NsxManager string
	NsxUser    string
	NsxPass    string
	Project    string
	SrcPod     string
	SrcPort    string
	DstPod     string
	DstPort    string
	Payload    string
	DstProject string
}

func validate(a *Args) bool {
	// Check for some values that could be env variables
	if a.NsxManager == "" {
		a.NsxManager = os.Getenv("NSX_MANAGER")
		if a.NsxManager == "" {
			return false
		}
	}
	if a.NsxUser == "admin" {
		a.NsxUser = os.Getenv("NSX_USER")
		if a.NsxUser == "" {
			a.NsxUser = "admin"
		}
	}
	if a.NsxPass == "" {
		a.NsxPass = os.Getenv("NSX_PASS")
		if a.NsxPass == "" {
			return false
		}
	}
	// Check the rest of the required variables
	if a.Project == "" || a.SrcPod == "" || a.SrcPort == "" || a.DstPod == "" || a.DstPort == "" {
		fmt.Println("Missing required variable")
	}

	return true
}

func RequiredVars() {
	fmt.Println("Required:")
	fmt.Println("	--nsx-ip ", "Or set NSX_MANAGER env variable")
	fmt.Println("	--nsx-pass ", "Or set NSX_PASS env variable")
	fmt.Println("	--namespace")
	fmt.Println("	--src-pod")
	fmt.Println("	--src-port")
	fmt.Println("	--dst-pod")
	fmt.Println("	--dst-port")
	fmt.Println("")
	fmt.Println("Optional:")
	fmt.Println("	--dst-namespace ", "If destination namespace is different than source")
	fmt.Println("	--payload ", "Optional text payload")
}

func main() {

	flag.Parse()
	a := Args{
		NsxManager: *nsx,
		NsxUser:    *user,
		Project:    *project,
		SrcPod:     *srcPod,
		SrcPort:    *srcPort,
		DstPod:     *dstPod,
		DstPort:    *dstPort,
		Payload:    *payload,
		NsxPass:    *pass,
		DstProject: *dstProject,
	}

	valid := validate(&a)
	if !valid {
		RequiredVars()
		os.Exit(1)
	}

	client := http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	sp, _ := strconv.Atoi(a.SrcPort)
	dp, _ := strconv.Atoi(a.DstPort)
	trace := TraceRequest{
		Timeout: 5000,
		Packet: Packet{
			ResourceType: "FieldsPacketData",
			//FrameSize:     128,
			Routed:        false,
			TransportType: "UNICAST",
			Payload:       a.Payload,
			TransportHeader: TransportHeader{
				TcpHeader{
					SrcPort:  sp,
					DstPort:  dp,
					TcpFlags: 2,
				},
			},
			IpHeader: IpHeader{
				Protocol: 6,
				Ttl:      64,
				Flags:    0,
			},
			EthHeader: EthHeader{
				EthType: "2048",
			},
		},
	}

	// Logical Ports
	srcLogicalPort, err := getLogicalPort(&client, a.SrcPod, a.Project, &a)
	if err != nil {
		log.Fatalln("Error getting source logical port: ", err)
	}
	trace.LogicalPortId = srcLogicalPort.Id
	trace.Packet.IpHeader.SrcIp = srcLogicalPort.AddressBindings[0]["ip_address"]
	trace.Packet.EthHeader.SrcMac = srcLogicalPort.AddressBindings[0]["mac_address"]

	if a.DstProject == "" {
		a.DstProject = a.Project
	}
	dstLogicalPort, err := getLogicalPort(&client, a.DstPod, a.DstProject, &a)
	if err != nil {
		log.Fatalln("Error getting destination logical port: ", err)
	}
	trace.Packet.IpHeader.DstIp = dstLogicalPort.AddressBindings[0]["ip_address"]
	trace.Packet.EthHeader.DstMac = dstLogicalPort.AddressBindings[0]["mac_address"]

	tr, err := PostTrace(&client, &trace, &a)
	if err != nil {
		log.Fatalln("Error during post: ", err)
	}

	fmt.Print("Traceflow in progress ")
	// Wait for Trace to finish
	for tr.OpState == "IN_PROGRESS" {
		fmt.Print(".")
		time.Sleep(1000 * time.Millisecond)
		err = GetTrace(&client, tr, &a)
		if err != nil {
			log.Fatalln("Error getting trace result: ", err)
		}
	}
	fmt.Println("")

	if tr.OpState != "FINISHED" {
		log.Fatalln("Traceroute did not finish correctly")
	}

	// Get the observations
	err = GetObservations(&client, tr, &a)
	if err != nil {
		log.Fatalln("Could not get observations: ", err)
	}

	dropped := false
	for _, observation := range tr.Observations {
		if observation.Reason == "FW_RULE" {
			dropped = true
			fmt.Println("Dropped Firewall RuleId: ", observation.ACLRuleId)
			// Maybe get firewall info
		}
	}

	if !dropped {
		fmt.Println("Delivered")
	}

}

func GetObservations(c *http.Client, tr *TraceResult, a *Args) error {
	url := "https://" + a.NsxManager + "/api/v1/traceflows/" + tr.Id + "/observations"
	h := HttpInfo{
		Client: c,
		Url:    url,
		Method: httpGet,
	}
	err := httpCall(&h)
	if err != nil {
		return err
	}
	var observations ObservationResult
	err = json.Unmarshal(h.Output, &observations)
	if err != nil {
		return err
	}
	for _, o := range observations.Results {
		tr.Observations = append(tr.Observations, o)
	}

	return nil
}

func GetTrace(c *http.Client, tr *TraceResult, a *Args) error {
	url := "https://" + a.NsxManager + "/api/v1/traceflows/" + tr.Id
	h := HttpInfo{
		Client: c,
		Url:    url,
		Method: httpGet,
	}
	err := httpCall(&h)
	if err != nil {
		return err
	}
	err = json.Unmarshal(h.Output, &tr)
	if err != nil {
		return err
	}
	return nil
}

func PostTrace(c *http.Client, request *TraceRequest, a *Args) (*TraceResult, error) {
	tr := TraceResult{}
	url := "https://" + a.NsxManager + "/api/v1/traceflows"
	t, err := json.Marshal(request)
	if err != nil {
		return &tr, err
	}
	h := HttpInfo{
		Client: c,
		Url:    url,
		Method: httpPost,
		Input:  t,
	}
	err = httpCall(&h)
	if err != nil {
		return &tr, err
	}
	err = json.Unmarshal(h.Output, &tr)
	if err != nil {
		return &tr, err
	}

	return &tr, nil
}

func getLogicalPort(c *http.Client, pod, nameSpace string, a *Args) (*LogicalPort, error) {

	var logicalPort *LogicalPort
	url := "https://" + a.NsxManager + "/api/v1/logical-ports/"
	h := HttpInfo{
		Client: c,
		Url:    url,
		Method: httpGet,
	}
	err := httpCall(&h)
	if err != nil {
		return logicalPort, err
	}

	var logicalPortsResult LogicalPortResult
	err = json.Unmarshal(h.Output, &logicalPortsResult)
	if err != nil {
		return logicalPort, err
	}

	for _, lp := range logicalPortsResult.Result {
		podCheck := false
		nsCheck := false
		// Check for tags
		for _, tag := range lp.Tags {
			if tag["scope"] == "ncp/pod" && tag["tag"] == pod {
				podCheck = true
			}
			if tag["scope"] == "ncp/project" && tag["tag"] == nameSpace {
				nsCheck = true
			}
		}
		if podCheck && nsCheck {
			logicalPort = &lp
			break
		}
	}

	return logicalPort, nil
}

type TraceResult struct {
	Id           string `json:"id,omitempty"`
	OpState      string `json:"operation_state,omitempty"`
	Observations []Observation
}

type LogicalPortResult struct {
	Result []LogicalPort `json:"results,omitempty"`
}

type LogicalPort struct {
	Id              string              `json:"id,omitempty"`
	DisplayName     string              `json:"display_name,omitempty"`
	AdminState      string              `json:"admin_state,omitempty"`
	Tags            []map[string]string `json:"tags,omitempty"`
	LogicalSwitchId string              `json:"logical_switch_id,omitempty"`
	AddressBindings []map[string]string `json:"address_bindings,omitempty"`
}

type TraceRequest struct {
	Timeout       int `json:"timeout,omitempty"`
	Packet        `json:"packet,omitempty"`
	LogicalPortId string `json:"lport_id,omitempty"`
}

type Param struct {
	Timeout       string `json:"timeout,omitempty"`
	LogicalPortId string `json:"lport_id,omitempty"`
	Packet        `json:"packet,omitempty"`
}

type Packet struct {
	ResourceType    string `json:"resource_type,omitempty"`
	Routed          bool   `json:"routed,omitempty"`
	TransportType   string `json:"transport_type,omitempty"`
	Payload         string `json:"payload,omitempty"`
	IpHeader        `json:"ip_header,omitempty"`
	EthHeader       `json:"eth_header,omitempty"`
	TransportHeader `json:"transport_header,omitempty"`
}

type IpHeader struct {
	SrcIp    string `json:"src_ip,omitempty"`
	DstIp    string `json:"dst_ip,omitempty"`
	Protocol int    `json:"protocol,omitempty"`
	Ttl      int    `json:"ttl,omitempty"`
	Flags    int    `json:"flags,omitempty"`
}

type EthHeader struct {
	SrcMac  string `json:"src_mac,omitempty"`
	DstMac  string `json:"dst_mac,omitempty"`
	EthType string `json:"eth_type,omitempty"`
}

type TransportHeader struct {
	TcpHeader `json:"tcp_header,omitempty"`
}

type TcpHeader struct {
	SrcPort  int `json:"src_port,omitempty"`
	DstPort  int `json:"dst_port,omitempty"`
	TcpFlags int `json:"tcp_flags,omitempty"`
}

type ObservationResult struct {
	Results []Observation `json:"results,omitempty"`
}

type Observation struct {
	ComponentType   string `json:"component_type,omitempty"`
	ComponentName   string `json:"component_name,omitempty"`
	ACLRuleId       int    `json:"acl_rule_id,omitempty"`
	LogicalPortId   string `json:"lport_id,omitempty"`
	LogicalPortName string `json:"lport_name,omitempty"`
	Reason          string `json:"reason,omitempty"`
}

func httpCall(h *HttpInfo) error {
	req, err := http.NewRequest(h.Method, h.Url, bytes.NewBuffer(h.Input))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.SetBasicAuth(*user, *pass)
	resp, err := h.Client.Do(req)
	if err != nil {
		return err
	}

	h.Output, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	resp.Body.Close()

	return nil
}

type HttpInfo struct {
	Client *http.Client
	Url    string
	Method string
	Input  []byte
	Output []byte
}
