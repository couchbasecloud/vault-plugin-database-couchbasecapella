package couchbasecapella

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"

	"github.com/couchbase/gocb/v2"
	"github.com/hashicorp/errwrap"
	"github.com/hashicorp/vault/sdk/database/helper/connutil"
	"github.com/mitchellh/mapstructure"
)

type couchbaseCapellaDBConnectionProducer struct {
	AccessKey            string `json:"access_key"`
	SecretKey            string `json:"secret_key"`
	ClusterID            string `json:"cluster_id"`
	ClusterType          string `json:"cluster_type"`
	CloudAPIBaseURL      string `json:"cloud_api_base_url"`
	CloudAPIClustersPath string `json:"cloud_api_clusters_path"`
	BucketName           string `json:"bucket_name"`
	ScopeName            string `json:"scope_name"`
	AccessRole           string `json:"access_role"`

	Hosts       string `json:"hosts"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	TLS         bool   `json:"tls"`
	InsecureTLS bool   `json:"insecure_tls"`
	Base64Pem   string `json:"base64pem"`

	Initialized bool
	rawConfig   map[string]interface{}
	Type        string
	cluster     *gocb.Cluster
	sync.RWMutex
}

func (c *couchbaseCapellaDBConnectionProducer) secretValues() map[string]string {
	return map[string]string{
		c.Password: "[password]",
		c.Username: "[username]",
	}
}

func (c *couchbaseCapellaDBConnectionProducer) Init(ctx context.Context, initConfig map[string]interface{}, verifyConnection bool) (saveConfig map[string]interface{}, err error) {
	// Don't let anyone read or write the config while we're using it
	c.Lock()
	defer c.Unlock()

	c.rawConfig = initConfig

	decoderConfig := &mapstructure.DecoderConfig{
		Result:           c,
		WeaklyTypedInput: true,
		TagName:          "json",
	}

	decoder, err := mapstructure.NewDecoder(decoderConfig)
	if err != nil {
		return nil, err
	}

	err = decoder.Decode(initConfig)
	if err != nil {
		return nil, err
	}

	switch {
	case len(c.ClusterID) == 0:
		return nil, fmt.Errorf("cluster_id cannot be empty")
	case len(c.AccessKey) == 0:
		return nil, fmt.Errorf("access_key cannot be empty")
	case len(c.SecretKey) == 0:
		return nil, fmt.Errorf("secret_key cannot be empty")
	}

	if len(c.CloudAPIBaseURL) == 0 {
		c.CloudAPIBaseURL = "https://cloudapi.cloud.couchbase.com"
	}
	if len(c.ClusterType) == 0 {
		c.ClusterType = "provisioned"
	}
	if len(c.CloudAPIClustersPath) == 0 && c.ClusterType == "provisioned" {
		c.CloudAPIClustersPath = "/v3/clusters"
	} else if len(c.CloudAPIClustersPath) == 0 && c.ClusterType == "invpc" {
		c.CloudAPIClustersPath = "/v2/clusters"
	}
	if len(c.AccessRole) == 0 {
		c.AccessRole = "data_writer"
	}
	if len(c.ScopeName) == 0 {
		c.ScopeName = "*"
	}

	if c.TLS {
		if len(c.Base64Pem) == 0 {
			return nil, fmt.Errorf("base64pem cannot be empty")
		}

		if !strings.HasPrefix(c.Hosts, "couchbases://") {
			return nil, fmt.Errorf("hosts list must start with couchbases:// for TLS connection")
		}
	}

	c.Initialized = true
	verifyConnection = false // TBD: Check the cluster status with public APIs and don't make the connection

	if verifyConnection {
		if _, err := c.Connection(ctx); err != nil {
			c.close()
			return nil, errwrap.Wrapf("error verifying connection: {{err}}", err)
		}
	}

	return initConfig, nil
}

func (c *couchbaseCapellaDBConnectionProducer) Initialize(ctx context.Context, config map[string]interface{}, verifyConnection bool) error {
	_, err := c.Init(ctx, config, verifyConnection)
	return err
}

func (c *couchbaseCapellaDBConnectionProducer) Connection(ctx context.Context) (interface{}, error) {
	// This is intentionally not grabbing the lock since the calling functions
	// (e.g. CreateUser) are claiming it.

	if !c.Initialized {
		return nil, connutil.ErrNotInitialized
	}

	if c.cluster != nil {
		return c.cluster, nil
	}
	var err error
	var sec gocb.SecurityConfig
	var pem []byte

	if c.TLS {
		pem, err = base64.StdEncoding.DecodeString(c.Base64Pem)
		if err != nil {
			return nil, errwrap.Wrapf("error decoding Base64Pem: {{err}}", err)
		}
		rootCAs := x509.NewCertPool()
		ok := rootCAs.AppendCertsFromPEM([]byte(pem))
		if !ok {
			return nil, fmt.Errorf("failed to parse root certificate")
		}
		sec = gocb.SecurityConfig{
			TLSRootCAs:    rootCAs,
			TLSSkipVerify: c.InsecureTLS,
		}
	}

	c.cluster, err = gocb.Connect(
		c.Hosts,
		gocb.ClusterOptions{
			Username:       c.Username,
			Password:       c.Password,
			SecurityConfig: sec,
		})
	if err != nil {
		return nil, errwrap.Wrapf("error in Connection: {{err}}", err)
	}

	// For databases 6.0 and earlier, we will need to open a `Bucket instance before connecting to any other
	// HTTP services such as UserManager.

	if c.BucketName != "" {
		bucket := c.cluster.Bucket(c.BucketName)
		// We wait until the bucket is definitely connected and setup.
		err = bucket.WaitUntilReady(computeTimeout(ctx), nil)
		if err != nil {
			return nil, errwrap.Wrapf("error in Connection waiting for bucket: {{err}}", err)
		}
	} else {
		err = c.cluster.WaitUntilReady(computeTimeout(ctx), nil)

		if err != nil {
			return nil, errwrap.Wrapf("error in Connection waiting for cluster: {{err}}", err)
		}
	}

	return c.cluster, nil
}

// close terminates the database connection without locking
func (c *couchbaseCapellaDBConnectionProducer) close() error {
	if c.cluster != nil {
		if err := c.cluster.Close(&gocb.ClusterCloseOptions{}); err != nil {
			return err
		}
	}

	c.cluster = nil
	return nil
}

// Close terminates the database connection with locking
func (c *couchbaseCapellaDBConnectionProducer) Close() error {
	// Don't let anyone read or write the config while we're using it
	c.Lock()
	defer c.Unlock()

	return c.close()
}
