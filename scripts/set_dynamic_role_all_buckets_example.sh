vault write database/roles/mydynamicrole3 db_name="couchbasecapella-database" creation_statements='{"access": [
		{
		  "privileges": [
			"data_reader"
		  ],
		  "resources": {
			"buckets": [
				{ "name": "*" }
			]
		  }
		}
	  ]}' default_ttl="5m" max_ttl="1h"
