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
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-version"

	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
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
	if resp.StatusCode != http.StatusOK {
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
	if resp.StatusCode != http.StatusOK {
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
	logger     hclog.Logger
}

func NewClient(baseURL, access, secret string) *CapellaClient {
	return &CapellaClient{
		baseURL:    baseURL,
		access:     access,
		secret:     secret,
		httpClient: http.DefaultClient,
		logger:     hclog.New(&hclog.LoggerOptions{}),
	}
}

func (c *CapellaClient) sendRequest(method string, url string, payload string) (*http.Response, error) {
	c.httpClient.Timeout = 30 * time.Second
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	//log.Printf("\n\n\t%s %s\n\tAuthorization: %s\n\t%s\n", method, url, authToken, payload)
	req, err := http.NewRequest(method, url, bytes.NewBuffer([]byte(payload)))
	if err != nil {
		fmt.Printf("client: could not create request: %s\n", err)
		fmt.Printf("error=%v", err)
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.secret))
	//req.Header.Set("X-forwarded-for", clientIP)
	if req.Method == http.MethodPost || req.Method == http.MethodPut {
		if strings.Contains(url, "?") {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		} else {
			req.Header.Set("Content-Type", "application/json")
		}
	}

	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	return c.httpClient.Do(req)

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

func CreateCapellaDbCredUser(baseUrl string, cloudAPIclustersEndPoint string, accessKey string, secretKey,
	username string, password string, access string) error {

	c := NewCapellaClient(baseUrl, accessKey, secretKey)
	if c == nil {
		return fmt.Errorf("failed in creating capella client, %v", c)
	}

	var accessdata map[string]interface{}
	err := json.Unmarshal([]byte(access), &accessdata)
	if err != nil {
		return fmt.Errorf("failed during capella user creation, unmarshal of access statement error = %v, user = %v, access statement=%v",
			err, username, access)
	}
	adata, err := json.Marshal(accessdata["access"])
	if err != nil {
		return fmt.Errorf("failed during capella user creation, marshal of access statement error = %v, user = %v, access statement=%v",
			err, username, accessdata["access"])
	}

	data := fmt.Sprintf("{\"name\":\"%s\", \"password\":\"%s\", \"access\":%v}", username, password, string(adata))

	ep := c.baseURL + cloudAPIclustersEndPoint + "/users"
	resp, err := c.sendRequest(http.MethodPost, ep, string(data))
	if resp != nil && resp.StatusCode != 201 {
		defer resp.Body.Close()
		// obfuscate password in the log
		obfData := fmt.Sprintf("{\"name\":\"%s\", \"password\":\"[password]\", \"access\":%v}", username, string(adata))
		b, err1 := io.ReadAll(resp.Body)
		if err1 != nil {
			return fmt.Errorf("failed during capella user creation, reading response error = %v, ep = %s, user = %v, payload=%v,client=%v",
				err1, ep, username, obfData, c)
		}
		return fmt.Errorf("failed during capella user creation, response = %s, ep = %s, user = %v, payload = %v, access=%s, secret=%s",
			string(b), ep, username, obfData, accessKey, secretKey)
	}
	if err != nil {
		return err
	}

	return nil
}

func UpdateCapellaDbCredUser(baseUrl string, cloudAPIclustersEndPoint string, accessKey string, secretKey, username string, password string) (string, error) {
	c := NewCapellaClient(baseUrl, accessKey, secretKey)

	if username != accessKey { // db cred update
		userId, err := getDbCredId(baseUrl, cloudAPIclustersEndPoint, accessKey, secretKey, username)
		if userId == "" || err != nil {
			return "", err
		}

		data := fmt.Sprintf("{\"password\":\"%s\"}", password)
		ep := c.baseURL + cloudAPIclustersEndPoint + "/users/" + userId
		resp, err := c.sendRequest(http.MethodPut, ep, data)
		if resp != nil && resp.StatusCode != http.StatusNoContent {
			return "", fmt.Errorf("failed during capella db cred user update, response = %v, ep = %s, payload=%s",
				resp, ep, data)
		}
		if err != nil {
			return "", err
		}

	} else { // secret key rotation
		apiPathSlices := strings.Split(cloudAPIclustersEndPoint, "/")
		ep := c.baseURL + "/organizations/" + apiPathSlices[2] + "/apikeys/" + username + "/rotate"
		data := fmt.Sprintf("{\"secret\":\"%s\"}", password)
		c.logger.Info(fmt.Sprintf("%s %s %s", http.MethodPost, ep, data))
		resp, err := c.sendRequest(http.MethodPost, ep, data)
		if resp != nil && resp.StatusCode != 201 {
			return "", fmt.Errorf("failed during capella secret key rotate, response = %v, ep = %s",
				resp, ep)
		}
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("failed during capella user id fetch unmarshal, response = %v, ep = %s, error=%v",
				resp, ep, err)
		}
		var content map[string]string
		err = json.Unmarshal([]byte(body), &content)
		if err != nil {
			return "", fmt.Errorf("failed during capella user id fetch unmarshal, response = %v, ep = %s, error=%v",
				resp, ep, err)
		}
		return content["secretKey"], nil
	}
	return "", nil
}

func DeleteCapellaDbCredUser(baseUrl string, cloudAPIclustersEndPoint string, accessKey string, secretKey, username string) error {
	c := NewCapellaClient(baseUrl, accessKey, secretKey)

	userId, err := getDbCredId(baseUrl, cloudAPIclustersEndPoint, accessKey, secretKey, username)
	if userId == "" || err != nil {
		return err
	}
	ep := c.baseURL + cloudAPIclustersEndPoint + "/users/" + userId
	resp, err := c.sendRequest(http.MethodDelete, ep, "")
	if resp != nil && resp.StatusCode != 204 {
		return fmt.Errorf("failed during capella user deletion, response = %v, ep = %s",
			resp, ep)
	}
	if err != nil {
		return err
	}
	return nil
}

type Hrefs struct {
	First    string `json:"first"`
	Last     string `json:"last"`
	Next     string `json:"next"`
	Previous string `json:"previous"`
}
type Pages struct {
	// Last Last page number.
	Last int `json:"last"`

	// Next Next page number, it is not set on the last page.
	Next *int `json:"next,omitempty"`

	// Page Current page starting from 1.
	Page int `json:"page"`

	// PerPage How many items are displayed in the page.
	PerPage int `json:"perPage"`

	// Previous Previous page number, it is not set on the first page.
	Previous *int `json:"previous,omitempty"`

	// TotalItems Total items found by the given query.
	TotalItems int `json:"totalItems"`
}
type Cursor struct {
	Hrefs Hrefs `json:"hrefs"`
	Pages Pages `json:"pages"`
}

type ListDbCredResponse struct {
	Cursor Cursor        `json: "cursor"`
	Data   []interface{} `json:"data"`
}

func getDbCredId(baseUrl string, cloudAPIclustersEndPoint string, accessKey string, secretKey, username string) (string, error) {
	c := NewCapellaClient(baseUrl, accessKey, secretKey)
	dbUserId := ""
	page := 1
	ep := fmt.Sprintf("%s%s/users?page=%d&perPage=100", c.baseURL, cloudAPIclustersEndPoint, page)
	for page > 0 {
		resp, _ := c.sendRequest(http.MethodGet, ep, "")
		if resp.StatusCode != http.StatusOK {
			return dbUserId, fmt.Errorf("failed during capella user id fetch, response = %v, ep = %s",
				resp, ep)
		} else {
			defer resp.Body.Close()

			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return dbUserId, fmt.Errorf("failed during capella user id fetch unmarshal, response = %v, ep = %s, error=%v",
					resp, ep, err)
			}
			var content ListDbCredResponse
			err = json.Unmarshal([]byte(body), &content)
			if err != nil {
				return dbUserId, fmt.Errorf("failed during capella user id fetch unmarshal, response = %v, ep = %s, error=%v",
					resp, ep, err)
			}
			d := content.Data

			if d == nil {
				return dbUserId, fmt.Errorf("failed during capella user id response data, response = %v, ep = %s, body=%v",
					resp, ep, body)
			}
			for _, data := range d {
				d1 := data.(map[string]interface{})
				dbusername := d1["name"].(string)
				if dbusername == username {
					dbUserId = d1["id"].(string)
					return dbUserId, nil
				}
			}
			// next page
			page = content.Cursor.Pages.Page
			if page == 0 {
				return dbUserId, fmt.Errorf("failed during capella user id fetch unmarshal, response = %v, ep = %s, error=%v",
					resp, ep, "db user id is not found for the given username")
			} else {
				ep = content.Cursor.Hrefs.Next
			}
		}
	}
	return dbUserId, nil
}
