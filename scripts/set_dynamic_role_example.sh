vault write database/roles/mydynamicrole1 db_name="couchbasecapella-database" creation_statements='{"access": [
		{
		  "privileges": [
			"data_reader", "data_writer"
		  ],
		  "resources": {
			"buckets": [
			  {
				"name": "vault-bucket-1",
				"scopes": [
				  {
					"name": "vault-bucket-1-scope-1",
					"collections": [
						"vault-bucket-1-scope-1-collect-1"
					]
				  }
				]
			  }
			]
		  }
		}
	  ]}' default_ttl="5m" max_ttl="1h"
