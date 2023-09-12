vault write database/roles/mydynamicrole2 db_name="couchbasecapella-database" creation_statements='{"access": [
		{
		  "privileges": [
			"data_reader", "data_writer"
		  ],
		  "resources": {
			"buckets": [
				{ 
					"name": "vault-bucket-1", 
					"scopes": [
						{ "name": "*" }
					]
				},
				{ 
					"name": "vault-bucket-2", 
					"scopes": [
						{ "name": "*" }
					]
				},
				{ 
					"name": "vault-bucket-3", 
					"scopes": [
						{ "name": "*" }
					]
				}
			]
		  }
		}
	  ]}' default_ttl="5m" max_ttl="1h"
