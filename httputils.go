package couchbasecapella

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/hashicorp/go-version"

	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"io"
	"strconv"
)

func CheckForOldCouchbaseCapellaVersion(hostname, username, password string) (is_old bool, err error) {

	//[TODO] handle list of hostnames

	resp, err := http.Get(fmt.Sprintf("http://%s:%s@%s:8091/pools", username, password, hostname))
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	type Pools struct {
		ImplementationVersion string `json:"implementationVersion"`
	}
	data := Pools{}
	err = json.Unmarshal(body, &data)
	if err != nil {
		return false, err
	}
	v, err := version.NewVersion(data.ImplementationVersion)

	v650, err := version.NewVersion("6.5.0-0000")
	if err != nil {
		return false, err
	}

	if v.LessThan(v650) {
		return true, nil
	}
	return false, nil

}

func getRootCAfromCouchbaseCapella(url string) (Base64pemCA string, err error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(body), nil
}

func createUser(hostname string, port int, adminuser, adminpassword, username, password, rbacName, roles string) (err error) {
	v := url.Values{}

	v.Set("password", password)
	v.Add("roles", roles)
	v.Add("name", rbacName)

	req, err := http.NewRequest(http.MethodPut,
		fmt.Sprintf("http://%s:%s@%s:%d/settings/rbac/users/local/%s",
			adminuser, adminpassword, hostname, port, username),
		strings.NewReader(v.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.Status != "200 OK" {
		return fmt.Errorf("createUser returned %s", resp.Status)
	}
	return nil
}

func createGroup(hostname string, port int, adminuser, adminpassword, group, roles string) (err error) {
	v := url.Values{}

	v.Set("roles", roles)

	req, err := http.NewRequest(http.MethodPut,
		fmt.Sprintf("http://%s:%s@%s:%d/settings/rbac/groups/%s",
			adminuser, adminpassword, hostname, port, group),
		strings.NewReader(v.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.Status != "200 OK" {
		return fmt.Errorf("createGroup returned %s", resp.Status)
	}
	return nil
}

func waitForBucket(t *testing.T, address, username, password, bucketName string) {
	t.Logf("Waiting for bucket %s...", bucketName)
	f := func() error {
		return checkBucketReady(address, username, password, bucketName)
	}
	bo := backoff.WithMaxRetries(backoff.NewConstantBackOff(1*time.Second), 10)
	err := backoff.Retry(f, bo)
	if err != nil {
		t.Fatalf("bucket %s installed check failed: %s", bucketName, err)
	}
}

func checkBucketReady(address, username, password, bucket string) (err error) {
	resp, err := http.Get(fmt.Sprintf("http://%s:%s@%s:8091/sampleBuckets", username, password, address))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	type installed []struct {
		Name        string `json:"name"`
		Installed   bool   `json:"installed"`
		QuotaNeeded int64  `json:"quotaNeeded"`
	}

	var iresult installed

	err = json.Unmarshal(body, &iresult)
	if err != nil {
		err := &backoff.PermanentError{
			Err: fmt.Errorf("error unmarshaling JSON %s", err),
		}
		return err
	}

	bucketFound := false
	for _, s := range iresult {
		if s.Name == bucket {
			bucketFound = true
			if s.Installed == true {
				return nil // Found & installed
			}
		}
	}

	err = fmt.Errorf("bucket not found")

	if !bucketFound {
		return backoff.Permanent(err)
	}
	return err
}

// Capella client utils
// --------------------
const (
	headerKeyTimestamp     = "Couchbase-Timestamp"
	headerKeyAuthorization = "Authorization"
	headerKeyContentType   = "Content-Type"
)

type CapellaClient struct {
	baseURL    string
	access     string
	secret     string
	httpClient *http.Client
}

func NewClient(baseURL, access, secret string) *CapellaClient {
	return &CapellaClient{
		baseURL:    baseURL,
		access:     access,
		secret:     secret,
		httpClient: http.DefaultClient,
	}
}

func (c *CapellaClient) Do(method, uri string, body interface{}) (*http.Response, error) {
	var bb io.Reader

	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal body: %w", err)
		}

		bb = bytes.NewReader(b)
	}

	r, err := http.NewRequest(method, c.baseURL+uri, bb)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	r.Header.Add(headerKeyContentType, "application/json")

	now := strconv.FormatInt(time.Now().Unix(), 10)
	r.Header.Add(headerKeyTimestamp, now)

	payload := strings.Join([]string{method, uri, now}, "\n")
	h := hmac.New(sha256.New, []byte(c.secret))
	h.Write([]byte(payload))

	bearer := "Bearer " + c.access + ":" + base64.StdEncoding.EncodeToString(h.Sum(nil))
	r.Header.Add(headerKeyAuthorization, bearer)

	return c.httpClient.Do(r)
}

// --
func NewCapellaClient(baseUrl string, accessKey string, secretKey string) *CapellaClient {
	return NewClient(baseUrl, accessKey, secretKey)
}

func Unmarshal(body io.Reader, v interface{}) error {
	rb, err := ioutil.ReadAll(body)
	if err != nil {
		return err
	}

	return json.Unmarshal(rb, v)
}

// --

type UserCreatePayload struct {
	Username         string             `json:"username,omitempty"`
	Password         string             `json:"password,omitempty"`
	AllBucketsAccess string             `json:"allBucketsAccess,omitempty"`
	Buckets          []UserCreateAccess `json:"buckets,omitempty"`
}

type UserCreateAccess struct {
	Name  string `json:"name"`
	Scope string `json:"scope"`
	Roles string `json:"access"`
}

func CreateCapellaUser(baseUrl string, clusterID string, accessKey string, secretKey,
	cloudAPIclustersEndPoint string, bucketName string, username string, password string, roleName string) error {

	c := NewCapellaClient(baseUrl, accessKey, secretKey)
	if c == nil {
		return fmt.Errorf("Failed in creating capella client, %v", c)
	}

	if roleName == "" || len(roleName) == 0 {
		roleName = "data_writer"
	}
	var userCreatePayload UserCreatePayload
	if bucketName != "" || len(bucketName) == 0 {
		userCreatePayload = UserCreatePayload{
			Username: username,
			Password: password,
			Buckets: []UserCreateAccess{
				{
					Name:  bucketName,
					Scope: "*",
					Roles: roleName,
				},
			},
		}
	} else {
		userCreatePayload = UserCreatePayload{
			AllBucketsAccess: roleName,
		}
	}

	resp, err := c.Do(http.MethodPost, cloudAPIclustersEndPoint+"/"+clusterID+"/users", userCreatePayload)
	if resp != nil && resp.StatusCode != 201 {
		defer resp.Body.Close()
		b, err1 := io.ReadAll(resp.Body)
		if err1 != nil {
			return fmt.Errorf("Failed during capella user creation, reading response error = %v, ep = %s, user = %v",
				err1, cloudAPIclustersEndPoint+"/"+clusterID+"/users", userCreatePayload.Username)
		}
		return fmt.Errorf("Failed during capella user creation, response = %s, ep = %s, user = %v",
			string(b), cloudAPIclustersEndPoint+"/"+clusterID+"/users", userCreatePayload.Username)
	}
	if err != nil {
		return err
	}

	return nil
}

func DeleteCapellaUser(baseUrl string, clusterID string, accessKey string, secretKey, cloudAPIclustersEndPoint string, username string) error {
	c := NewCapellaClient(baseUrl, accessKey, secretKey)
	resp, err := c.Do(http.MethodDelete, cloudAPIclustersEndPoint+"/"+clusterID+"/users/"+username, nil)
	if resp != nil && resp.StatusCode != 204 {
		return fmt.Errorf("Failed during capella user deletion, response = %v, ep = %s",
			resp, cloudAPIclustersEndPoint+"/"+clusterID+"/users/"+username)
	}
	if err != nil {
		return err
	}
	return nil
}
