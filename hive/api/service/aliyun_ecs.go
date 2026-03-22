package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ECSService manages Aliyun ECS instances for the "pro" and "enterprise" deploy modes.
type ECSService struct {
	accessKeyID     string
	accessKeySecret string
	regionID        string
	vpcID           string
	vSwitchID       string
	securityGroupID string
	imageID         string // Claw pre-baked image
	client          *http.Client
}

type ECSConfig struct {
	AccessKeyID     string
	AccessKeySecret string
	RegionID        string // e.g. cn-shanghai
	VPCID           string
	VSwitchID       string
	SecurityGroupID string
	ImageID         string // AMI with Claw pre-installed
}

func NewECSService(cfg ECSConfig) *ECSService {
	return &ECSService{
		accessKeyID:     cfg.AccessKeyID,
		accessKeySecret: cfg.AccessKeySecret,
		regionID:        cfg.RegionID,
		vpcID:           cfg.VPCID,
		vSwitchID:       cfg.VSwitchID,
		securityGroupID: cfg.SecurityGroupID,
		imageID:         cfg.ImageID,
		client:          &http.Client{Timeout: 60 * time.Second},
	}
}

// CreateInstanceResult from Aliyun CreateInstance API
type CreateInstanceResult struct {
	InstanceID string `json:"InstanceId"`
	RequestID  string `json:"RequestId"`
}

// InstanceInfo from Aliyun DescribeInstances API
type InstanceInfo struct {
	InstanceID   string `json:"InstanceId"`
	Status       string `json:"Status"` // Running, Stopped, Starting, Stopping
	PublicIP     string `json:"PublicIp"`
	PrivateIP    string `json:"PrivateIp"`
	InstanceType string `json:"InstanceType"`
}

// CreateInstance creates an ECS instance with the specified resources.
func (s *ECSService) CreateInstance(slug string, cpu float64, memoryMB, bandwidthMB int) (*CreateInstanceResult, error) {
	instanceType := s.resolveInstanceType(cpu, memoryMB)

	params := map[string]string{
		"Action":              "RunInstances",
		"RegionId":            s.regionID,
		"ImageId":             s.imageID,
		"InstanceType":        instanceType,
		"SecurityGroupId":     s.securityGroupID,
		"VSwitchId":           s.vSwitchID,
		"InstanceName":        fmt.Sprintf("claw-%s", slug),
		"HostName":            fmt.Sprintf("claw-%s", slug),
		"InternetChargeType":  "PayByTraffic",
		"InternetMaxBandwidthOut": fmt.Sprintf("%d", bandwidthMB),
		"InstanceChargeType":  "PostPaid", // pay-as-you-go
		"Amount":              "1",
		"SystemDisk.Category": "cloud_essd",
		"SystemDisk.Size":     "40",
		"Tag.1.Key":           "hive",
		"Tag.1.Value":         slug,
	}

	body, err := s.callAPI(params)
	if err != nil {
		return nil, fmt.Errorf("create ECS: %w", err)
	}

	var resp struct {
		InstanceIdSets struct {
			InstanceIdSet []string `json:"InstanceIdSet"`
		} `json:"InstanceIdSets"`
		RequestID string `json:"RequestId"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse ECS response: %w", err)
	}

	if len(resp.InstanceIdSets.InstanceIdSet) == 0 {
		return nil, fmt.Errorf("no instance ID returned")
	}

	return &CreateInstanceResult{
		InstanceID: resp.InstanceIdSets.InstanceIdSet[0],
		RequestID:  resp.RequestID,
	}, nil
}

// AllocatePublicIP assigns a public IP to the ECS instance.
func (s *ECSService) AllocatePublicIP(instanceID string) (string, error) {
	params := map[string]string{
		"Action":     "AllocatePublicIpAddress",
		"InstanceId": instanceID,
	}
	body, err := s.callAPI(params)
	if err != nil {
		return "", fmt.Errorf("allocate IP: %w", err)
	}
	var resp struct {
		IpAddress string `json:"IpAddress"`
	}
	json.Unmarshal(body, &resp)
	return resp.IpAddress, nil
}

// StartInstance starts an ECS instance.
func (s *ECSService) StartInstance(instanceID string) error {
	_, err := s.callAPI(map[string]string{
		"Action":     "StartInstance",
		"InstanceId": instanceID,
	})
	return err
}

// StopInstance stops an ECS instance.
func (s *ECSService) StopInstance(instanceID string) error {
	_, err := s.callAPI(map[string]string{
		"Action":     "StopInstance",
		"InstanceId": instanceID,
		"ForceStop":  "false",
	})
	return err
}

// DeleteInstance terminates and deletes an ECS instance.
func (s *ECSService) DeleteInstance(instanceID string) error {
	_, err := s.callAPI(map[string]string{
		"Action":     "DeleteInstance",
		"InstanceId": instanceID,
		"Force":      "true",
	})
	return err
}

// DescribeInstance gets the current status of an ECS instance.
func (s *ECSService) DescribeInstance(instanceID string) (*InstanceInfo, error) {
	params := map[string]string{
		"Action":     "DescribeInstances",
		"RegionId":   s.regionID,
		"InstanceIds": fmt.Sprintf(`["%s"]`, instanceID),
	}
	body, err := s.callAPI(params)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Instances struct {
			Instance []struct {
				InstanceId       string `json:"InstanceId"`
				Status           string `json:"Status"`
				PublicIpAddress  struct {
					IpAddress []string `json:"IpAddress"`
				} `json:"PublicIpAddress"`
				InnerIpAddress struct {
					IpAddress []string `json:"IpAddress"`
				} `json:"InnerIpAddress"`
				InstanceType string `json:"InstanceType"`
			} `json:"Instance"`
		} `json:"Instances"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	if len(resp.Instances.Instance) == 0 {
		return nil, fmt.Errorf("instance %s not found", instanceID)
	}

	inst := resp.Instances.Instance[0]
	info := &InstanceInfo{
		InstanceID:   inst.InstanceId,
		Status:       inst.Status,
		InstanceType: inst.InstanceType,
	}
	if len(inst.PublicIpAddress.IpAddress) > 0 {
		info.PublicIP = inst.PublicIpAddress.IpAddress[0]
	}
	if len(inst.InnerIpAddress.IpAddress) > 0 {
		info.PrivateIP = inst.InnerIpAddress.IpAddress[0]
	}
	return info, nil
}

// resolveInstanceType maps CPU/memory to an Aliyun instance type.
func (s *ECSService) resolveInstanceType(cpu float64, memoryMB int) string {
	memGB := memoryMB / 1024
	switch {
	case cpu <= 1 && memGB <= 1:
		return "ecs.t6-c1m1.large" // 1C1G burstable
	case cpu <= 1 && memGB <= 2:
		return "ecs.t6-c1m2.large" // 1C2G burstable
	case cpu <= 2 && memGB <= 4:
		return "ecs.c7.large" // 2C4G compute
	case cpu <= 4 && memGB <= 8:
		return "ecs.c7.xlarge" // 4C8G compute
	default:
		return "ecs.c7.2xlarge" // 8C16G compute
	}
}

// ──── Aliyun API v1 Signature ────

func (s *ECSService) callAPI(params map[string]string) ([]byte, error) {
	// Common parameters
	params["Format"] = "JSON"
	params["Version"] = "2014-05-26"
	params["AccessKeyId"] = s.accessKeyID
	params["SignatureMethod"] = "HMAC-SHA256"
	params["SignatureVersion"] = "1.0"
	params["SignatureNonce"] = uuid.New().String()
	params["Timestamp"] = time.Now().UTC().Format("2006-01-02T15:04:05Z")

	// Build signature
	params["Signature"] = s.sign(params)

	// Build URL
	values := url.Values{}
	for k, v := range params {
		values.Set(k, v)
	}

	resp, err := s.client.Get("https://ecs.aliyuncs.com/?" + values.Encode())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Code    string `json:"Code"`
			Message string `json:"Message"`
		}
		json.Unmarshal(body, &errResp)
		return nil, fmt.Errorf("aliyun ECS %s: %s", errResp.Code, errResp.Message)
	}

	return body, nil
}

func (s *ECSService) sign(params map[string]string) string {
	// Sort keys
	keys := make([]string, 0, len(params))
	for k := range params {
		if k == "Signature" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Canonicalize query string
	var pairs []string
	for _, k := range keys {
		pairs = append(pairs, url.QueryEscape(k)+"="+url.QueryEscape(params[k]))
	}
	canonicalized := strings.Join(pairs, "&")

	// String to sign
	stringToSign := "GET&" + url.QueryEscape("/") + "&" + url.QueryEscape(canonicalized)

	// HMAC-SHA256
	mac := hmac.New(sha256.New, []byte(s.accessKeySecret+"&"))
	mac.Write([]byte(stringToSign))
	return hex.EncodeToString(mac.Sum(nil))
}
