package couchbasecapella

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/couchbase/gocb/v2"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-secure-stdlib/strutil"
	dbplugin "github.com/hashicorp/vault/sdk/database/dbplugin/v5"
	"github.com/hashicorp/vault/sdk/database/helper/credsutil"
	"github.com/hashicorp/vault/sdk/helper/template"
)

const (
	couchbaseCapellaTypeName        = "couchbasecapella"
	defaultCouchbaseCapellaUserRole = `{"access": [{
		  "privileges": [
			"data_reader"
		  ],
		  "resources": {
			"buckets": [
				{ "name" :"*" }
			]
		  }
		  }]}`
	defaultTimeout = 20000 * time.Millisecond

	defaultUserNameTemplate = `{{printf "V_%s_%s_%s_%s" (printf "%s" .DisplayName | uppercase | truncate 64) (printf "%s" .RoleName | uppercase | truncate 64) (random 20 | uppercase) (unix_time) | truncate 128}}`
)

var _ dbplugin.Database = &CouchbaseCapellaDB{}
var logger hclog.Logger

// Type that combines the custom plugins Couchbase Capella database connection configuration options and the Vault CredentialsProducer
// used for generating user information for the Couchbase Capella database.
type CouchbaseCapellaDB struct {
	*couchbaseCapellaDBConnectionProducer
	credsutil.CredentialsProducer
	usernameProducer template.StringTemplate
	logger           hclog.Logger
}

// New implements builtinplugins.BuiltinFactory
func New() (interface{}, error) {
	db := new()
	// Wrap the plugin with middleware to sanitize errors
	dbType := dbplugin.NewDatabaseErrorSanitizerMiddleware(db, db.secretValues)
	return dbType, nil
}

func new() *CouchbaseCapellaDB {
	connProducer := &couchbaseCapellaDBConnectionProducer{}
	connProducer.Type = couchbaseCapellaTypeName

	db := &CouchbaseCapellaDB{
		couchbaseCapellaDBConnectionProducer: connProducer,
		logger:                               hclog.New(&hclog.LoggerOptions{}),
	}
	logger = hclog.New(&hclog.LoggerOptions{})

	return db
}

func (c *CouchbaseCapellaDB) Initialize(ctx context.Context, req dbplugin.InitializeRequest) (dbplugin.InitializeResponse, error) {
	usernameTemplate, err := strutil.GetString(req.Config, "username_template")
	if err != nil {
		return dbplugin.InitializeResponse{}, fmt.Errorf("failed to retrieve username_template: %w", err)
	}
	if usernameTemplate == "" {
		usernameTemplate = defaultUserNameTemplate
	}

	up, err := template.NewTemplate(template.Template(usernameTemplate))
	if err != nil {
		return dbplugin.InitializeResponse{}, fmt.Errorf("unable to initialize username template: %w", err)
	}
	c.usernameProducer = up

	err = c.couchbaseCapellaDBConnectionProducer.Initialize(ctx, req.Config, req.VerifyConnection)
	if err != nil {
		return dbplugin.InitializeResponse{}, err
	}
	resp := dbplugin.InitializeResponse{
		Config: req.Config,
	}

	c.logger.Info(fmt.Sprintf("Initialize, resp=%v", resp))
	return resp, nil
}

func (c *CouchbaseCapellaDB) NewUser(ctx context.Context, req dbplugin.NewUserRequest) (dbplugin.NewUserResponse, error) {
	// Don't let anyone write the config while we're using it
	c.RLock()
	defer c.RUnlock()

	username, err := c.usernameProducer.Generate(req.UsernameConfig)
	if err != nil {
		return dbplugin.NewUserResponse{}, fmt.Errorf("failed to generate username: %w", err)
	}
	username = strings.ToUpper(username)

	err = newUser(ctx, c.couchbaseCapellaDBConnectionProducer, username, req)
	if err != nil {
		return dbplugin.NewUserResponse{}, err
	}

	resp := dbplugin.NewUserResponse{
		Username: username,
	}

	return resp, nil
}

func callVaultAPI(method, path string, requestData map[string]interface{}, responseData interface{}) error {
	vaultAddr := os.Getenv("VAULT_ADDR")
	if len(vaultAddr) == 0 {
		vaultAddr = "http://127.0.0.1:8200"
	}
	vaultToken := os.Getenv("VAULT_TOKEN")
	if len(vaultToken) == 0 {
		vaultToken = os.Getenv("VAULT_DEV_ROOT_TOKEN_ID")
		if len(vaultToken) == 0 {
			vaultToken = "root"
		}
	}
	url := fmt.Sprintf("%s/v1/%s", vaultAddr, path)
	reqData, err := json.Marshal(requestData)
	if err != nil {
		return err
	}

	logger.Info(fmt.Sprintf("Attempting HTTP : %s %s", method, url))
	req, err := http.NewRequest(method, url, bytes.NewBuffer(reqData))
	if err != nil {
		return err
	}
	req.Header.Add("X-Vault-Token", vaultToken)

	client := http.Client{}
	defer client.CloseIdleConnections()
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if responseData != nil {
		if err := json.NewDecoder(resp.Body).Decode(&responseData); err != nil {
			return err
		}
	}

	return nil
}

func (c *CouchbaseCapellaDB) UpdateUser(ctx context.Context, req dbplugin.UpdateUserRequest) (dbplugin.UpdateUserResponse, error) {
	if req.Password != nil {
		newpassword := req.Password.NewPassword
		pwd, err := c.changeUserPassword(ctx, req.Username, newpassword)
		c.logger.Info("after change pwd=%v", pwd)
		return dbplugin.UpdateUserResponse{}, err
	}
	return dbplugin.UpdateUserResponse{}, nil
}

func (c *CouchbaseCapellaDB) DeleteUser(ctx context.Context, req dbplugin.DeleteUserRequest) (dbplugin.DeleteUserResponse, error) {
	// Don't let anyone write the config while we're using it
	c.RLock()
	defer c.RUnlock()

	err := DeleteCapellaDbCredUser(c.CloudAPIBaseURL, c.couchbaseCapellaDBConnectionProducer.CloudAPIClustersPath,
		c.couchbaseCapellaDBConnectionProducer.Username,
		c.couchbaseCapellaDBConnectionProducer.Password,
		req.Username)
	if err != nil {
		return dbplugin.DeleteUserResponse{}, err
	}

	return dbplugin.DeleteUserResponse{}, nil
}

func newUser(ctx context.Context, c *couchbaseCapellaDBConnectionProducer, username string, req dbplugin.NewUserRequest) error {
	statements := removeEmpty(req.Statements.Commands)
	if len(statements) == 0 {
		statements = append(statements, defaultCouchbaseCapellaUserRole)
	}

	c.logger.Info(fmt.Sprintf("%s :: %s :: %s :: %s :: %s :: %s", c.CloudAPIBaseURL, c.CloudAPIClustersPath, c.Username, c.Password,
		username, req.Password))

	err := CreateCapellaDbCredUser(c.CloudAPIBaseURL, c.CloudAPIClustersPath, c.Username, c.Password,
		username, req.Password, statements[0])
	if err != nil {
		return err
	}

	return nil
}

func (c *CouchbaseCapellaDB) changeUserPassword(ctx context.Context, username, password string) (string, error) {
	// Don't let anyone write the config while we're using it
	c.RLock()
	defer c.RUnlock()
	c.logger.Info(fmt.Sprintf("%s :: %s :: %s :: %s :: %s :: %s", c.CloudAPIBaseURL, c.CloudAPIClustersPath, c.Username, c.Password,
		username, password))
	pwd, err := UpdateCapellaDbCredUser(c.CloudAPIBaseURL, c.CloudAPIClustersPath, c.Username, c.Password,
		username, password)

	c.logger.Info(fmt.Sprintf("changeUserPassword, new password: %s", pwd))
	c.logger.Info(fmt.Sprintf("changeUserPassword, after change secretValues %v", c.secretValues()))
	if err != nil {
		return pwd, err
	}

	return pwd, nil
}

func removeEmpty(strs []string) []string {
	var newStrs []string
	for _, str := range strs {
		str = strings.TrimSpace(str)
		if str == "" {
			continue
		}
		newStrs = append(newStrs, str)
	}

	return newStrs
}

func computeTimeout(ctx context.Context) (timeout time.Duration) {
	deadline, ok := ctx.Deadline()
	if ok {
		return time.Until(deadline)
	}
	return defaultTimeout
}

func (c *CouchbaseCapellaDB) getConnection(ctx context.Context) (*gocb.Cluster, error) {
	db, err := c.Connection(ctx)
	if err != nil {
		return nil, err
	}
	return db.(*gocb.Cluster), nil
}

func (c *CouchbaseCapellaDB) Type() (string, error) {
	return couchbaseCapellaTypeName, nil
}
