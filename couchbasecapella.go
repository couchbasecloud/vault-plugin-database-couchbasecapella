package couchbasecapella

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/couchbase/gocb/v2"
	"github.com/hashicorp/errwrap"
	"github.com/hashicorp/go-secure-stdlib/strutil"
	dbplugin "github.com/hashicorp/vault/sdk/database/dbplugin/v5"
	"github.com/hashicorp/vault/sdk/database/helper/credsutil"
	"github.com/hashicorp/vault/sdk/helper/template"
)

const (
	couchbaseCapellaTypeName        = "couchbasecapella"
	defaultCouchbaseCapellaUserRole = `{"Roles": [{"role":"ro_admin"}]}`
	defaultTimeout                  = 20000 * time.Millisecond

	defaultUserNameTemplate = `{{printf "V_%s_%s_%s_%s" (printf "%s" .DisplayName | uppercase | truncate 64) (printf "%s" .RoleName | uppercase | truncate 64) (random 20 | uppercase) (unix_time) | truncate 128}}`
)

var _ dbplugin.Database = &CouchbaseCapellaDB{}

// Type that combines the custom plugins Couchbase Capella database connection configuration options and the Vault CredentialsProducer
// used for generating user information for the Couchbase Capella database.
type CouchbaseCapellaDB struct {
	*couchbaseCapellaDBConnectionProducer
	credsutil.CredentialsProducer

	usernameProducer template.StringTemplate
}

// Type that combines the Couchbase Capella Roles and Groups representing specific account permissions. Used to pass roles and or
// groups between the Vault server and the custom plugin in the dbplugin.Statements
type RolesAndGroups struct {
	Roles  []gocb.Role `json:"roles"`
	Groups []string    `json:"groups"`
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
	}

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

func (c *CouchbaseCapellaDB) UpdateUser(ctx context.Context, req dbplugin.UpdateUserRequest) (dbplugin.UpdateUserResponse, error) {
	if req.Password != nil {
		err := c.changeUserPassword(ctx, req.Username, req.Password.NewPassword)
		return dbplugin.UpdateUserResponse{}, err
	}
	return dbplugin.UpdateUserResponse{}, nil
}

func (c *CouchbaseCapellaDB) DeleteUser(ctx context.Context, req dbplugin.DeleteUserRequest) (dbplugin.DeleteUserResponse, error) {
	// Don't let anyone write the config while we're using it
	c.RLock()
	defer c.RUnlock()

	err := DeleteCapellaUser("https://cloudapi.cloud.couchbase.com", c.couchbaseCapellaDBConnectionProducer.ClusterID,
		c.couchbaseCapellaDBConnectionProducer.AccessKey,
		c.couchbaseCapellaDBConnectionProducer.SecretKey,
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

	jsonRoleAndGroupData := []byte(statements[0])

	var rag RolesAndGroups

	err := json.Unmarshal(jsonRoleAndGroupData, &rag)
	if err != nil {
		return errwrap.Wrapf("error unmarshalling roles and groups creation statement JSON: {{err}}", err)
	}

	err = CreateCapellaUser("https://cloudapi.cloud.couchbase.com", c.ClusterID, c.AccessKey, c.SecretKey, "*", username, req.Password, "")
	if err != nil {
		return err
	}

	return nil
}

func (c *CouchbaseCapellaDB) changeUserPassword(ctx context.Context, username, password string) error {
	// Don't let anyone write the config while we're using it
	c.RLock()
	defer c.RUnlock()

	db, err := c.getConnection(ctx)
	if err != nil {
		return err
	}

	// Get the UserManager
	mgr := db.Users()
	user, err := mgr.GetUser(username, nil)
	if err != nil {
		return fmt.Errorf("unable to retrieve user %s: %w", username, err)
	}
	user.User.Password = password

	err = mgr.UpsertUser(user.User,
		&gocb.UpsertUserOptions{
			Timeout:    computeTimeout(ctx),
			DomainName: "local",
		})

	if err != nil {
		return err
	}

	return nil
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
