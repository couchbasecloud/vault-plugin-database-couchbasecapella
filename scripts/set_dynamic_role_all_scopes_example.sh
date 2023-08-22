vault write database/roles/mydynamicrole2 db_name="couchbasecapella-database" creation_statements='{"access": [
		{
		  "privileges": [
			"data_reader", "data_writer"
		  ],
		  "resources": {
			"buckets": [
				{ 
					"name": "db-cred-test-12Qj", 
					"scopes": [
						{ "name": "*" }
					]
				},
				{ 
					"name": "db-cred-test-3zRb", 
					"scopes": [
						{ "name": "*" }
					]
				},
				{ 
					"name": "db-cred-test-FcAv", 
					"scopes": [
						{ "name": "*" }
					]
				}
			]
		  }
		}
	  ]}' default_ttl="5m" max_ttl="1h"
